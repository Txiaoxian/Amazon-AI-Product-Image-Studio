package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"golang.org/x/term"
	"gorm.io/gorm"
)

const (
	tenantNameEnv        = "PROVISION_TENANT_NAME"
	adminEmailEnv        = "PROVISION_TENANT_ADMIN_EMAIL"
	adminDisplayNameEnv  = "PROVISION_TENANT_ADMIN_DISPLAY_NAME"
	adminPasswordEnv     = "PROVISION_TENANT_ADMIN_PASSWORD"
	confirmationEnv      = "PROVISION_TENANT_CONFIRM"
	requiredConfirmation = "I_UNDERSTAND_TENANT_PROVISIONING"
	maxPasswordBytes     = 128
)

type commandDependencies struct {
	getenv          func(string) string
	stdin           io.Reader
	stdout          io.Writer
	stderr          io.Writer
	loadConfig      func() (config.Config, error)
	openDatabase    func(context.Context, config.DatabaseConfig) (*gorm.DB, error)
	runMigrations   func(context.Context, *gorm.DB) error
	closeDatabase   func(*gorm.DB) error
	provisionTenant func(context.Context, *gorm.DB, auth.TenantProvisioningInput) (auth.TenantProvisioningResult, error)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	os.Exit(run(ctx, os.Args[1:], defaultDependencies()))
}

func defaultDependencies() commandDependencies {
	return commandDependencies{
		getenv:          os.Getenv,
		stdin:           os.Stdin,
		stdout:          os.Stdout,
		stderr:          os.Stderr,
		loadConfig:      config.Load,
		openDatabase:    database.Open,
		runMigrations:   database.RunMigrations,
		closeDatabase:   database.Close,
		provisionTenant: auth.ProvisionTenant,
	}
}

func run(ctx context.Context, args []string, deps commandDependencies) int {
	flags := flag.NewFlagSet("provision-tenant", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	apply := flags.Bool("apply", false, "apply tenant provisioning")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(deps.stdout, "usage: provision-tenant [--apply]")
			return 0
		}
		fmt.Fprintln(deps.stderr, "tenant provisioning refused: invalid arguments")
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(deps.stderr, "tenant provisioning refused: invalid arguments")
		return 1
	}
	if *apply && deps.getenv(confirmationEnv) != requiredConfirmation {
		fmt.Fprintln(deps.stderr, "tenant provisioning refused: explicit apply confirmation is required")
		return 1
	}

	input, err := tenantProvisioningInput(deps)
	if err != nil || auth.ValidateTenantProvisioningInput(input) != nil {
		fmt.Fprintln(deps.stderr, "tenant provisioning input is invalid")
		return 1
	}
	if !*apply {
		fmt.Fprintln(deps.stdout, "tenant provisioning dry-run validated; no changes applied")
		return 0
	}

	cfg, err := deps.loadConfig()
	if err != nil {
		fmt.Fprintln(deps.stderr, "tenant provisioning failed")
		return 1
	}
	db, err := deps.openDatabase(ctx, cfg.Database)
	if err != nil {
		fmt.Fprintln(deps.stderr, "tenant provisioning failed")
		return 1
	}
	if err := deps.runMigrations(ctx, db); err != nil {
		_ = deps.closeDatabase(db)
		fmt.Fprintln(deps.stderr, "tenant provisioning failed")
		return 1
	}

	result, provisionErr := deps.provisionTenant(ctx, db, input)
	closeErr := deps.closeDatabase(db)
	if provisionErr != nil || closeErr != nil {
		fmt.Fprintln(deps.stderr, "tenant provisioning failed")
		return 1
	}

	fmt.Fprintf(deps.stdout, "tenant provisioning applied tenant_id=%s\n", result.TenantID)
	return 0
}

func tenantProvisioningInput(deps commandDependencies) (auth.TenantProvisioningInput, error) {
	password := deps.getenv(adminPasswordEnv)
	if password == "" {
		var err error
		password, err = readPassword(deps.stdin)
		if err != nil {
			return auth.TenantProvisioningInput{}, err
		}
	}

	return auth.TenantProvisioningInput{
		TenantName:       deps.getenv(tenantNameEnv),
		AdminEmail:       deps.getenv(adminEmailEnv),
		AdminDisplayName: deps.getenv(adminDisplayNameEnv),
		AdminPassword:    password,
	}, nil
}

func readPassword(reader io.Reader) (string, error) {
	var (
		password []byte
		err      error
	)
	if file, ok := reader.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		password, err = term.ReadPassword(int(file.Fd()))
	} else {
		password, err = io.ReadAll(io.LimitReader(reader, maxPasswordBytes+3))
	}
	if err != nil {
		return "", err
	}

	value := strings.TrimSuffix(string(password), "\n")
	value = strings.TrimSuffix(value, "\r")
	if len(value) > maxPasswordBytes || strings.ContainsAny(value, "\r\n") {
		return "", errors.New("invalid password input")
	}
	return value, nil
}
