package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"gorm.io/gorm"
)

const (
	providerKeyRotationOldSecretEnv = "PROVIDER_KEY_ROTATION_OLD_SECRET"
	providerKeyRotationOldKeyIDEnv  = "PROVIDER_KEY_ROTATION_OLD_KEY_ID"
	providerKeyRotationNewSecretEnv = "PROVIDER_KEY_ROTATION_NEW_SECRET"
	providerKeyRotationNewKeyIDEnv  = "PROVIDER_KEY_ROTATION_NEW_KEY_ID"
	providerKeyRotationConfirmEnv   = "PROVIDER_KEY_ROTATION_CONFIRM"
	providerKeyRotationConfirmToken = "I_UNDERSTAND_PROVIDER_KEY_ROTATION"
)

var (
	errRotationConfigurationInvalid = errors.New("provider key rotation configuration invalid")
	errRotationConfirmationRequired = errors.New("provider key rotation apply confirmation required")
	errRotationDatabaseUnavailable  = errors.New("provider key rotation database unavailable")
	errRotationExecutionFailed      = errors.New("provider key rotation execution failed")
	errRotationDatabaseCloseFailed  = errors.New("provider key rotation database close failed")
	errRotationOutputFailed         = errors.New("provider key rotation output failed")
)

type rotationOptions struct {
	oldCipher provider.APIKeyCipher
	newCipher provider.APIKeyCipher
	apply     bool
}

type commandDependencies struct {
	loadConfig    func() (config.Config, error)
	openDatabase  func(context.Context, config.DatabaseConfig) (*gorm.DB, error)
	closeDatabase func(*gorm.DB) error
	rotate        func(context.Context, *gorm.DB, provider.APIKeyCipher, provider.APIKeyCipher, bool) (provider.APIKeyRotationSummary, error)
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Getenv, os.Stdout, commandDependencies{}); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, output io.Writer, deps commandDependencies) error {
	options, err := rotationOptionsFromEnv(args, getenv)
	if err != nil {
		return err
	}
	deps = deps.withDefaults()

	cfg, err := deps.loadConfig()
	if err != nil {
		return errRotationConfigurationInvalid
	}
	if ctx == nil {
		ctx = context.Background()
	}
	connectCtx, cancel := context.WithTimeout(ctx, cfg.Database.ConnectTimeout)
	defer cancel()

	db, err := deps.openDatabase(connectCtx, cfg.Database)
	if err != nil {
		return errRotationDatabaseUnavailable
	}
	summary, rotateErr := deps.rotate(ctx, db, options.oldCipher, options.newCipher, options.apply)
	closeErr := deps.closeDatabase(db)
	if rotateErr != nil {
		return errRotationExecutionFailed
	}
	if closeErr != nil {
		return errRotationDatabaseCloseFailed
	}

	if output == nil {
		return errRotationOutputFailed
	}
	mode := "dry-run"
	if summary.Applied {
		mode = "apply"
	}
	if _, err := fmt.Fprintf(output, "provider key rotation mode=%s providers=%d deleted_provider_erase_candidates=%d result=success\n", mode, summary.ProviderCount, summary.DeletedProviderEraseCount); err != nil {
		return errRotationOutputFailed
	}
	return nil
}

func rotationOptionsFromEnv(args []string, getenv func(string) string) (rotationOptions, error) {
	if getenv == nil {
		return rotationOptions{}, errRotationConfigurationInvalid
	}

	flags := flag.NewFlagSet("provider-key-rotation", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	apply := flags.Bool("apply", false, "")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return rotationOptions{}, errRotationConfigurationInvalid
	}

	oldSecret := getenv(providerKeyRotationOldSecretEnv)
	oldKeyID := strings.TrimSpace(getenv(providerKeyRotationOldKeyIDEnv))
	newSecret := getenv(providerKeyRotationNewSecretEnv)
	newKeyID := strings.TrimSpace(getenv(providerKeyRotationNewKeyIDEnv))
	oldCipher, err := provider.NewAPIKeyCipher(oldSecret, oldKeyID)
	if err != nil {
		return rotationOptions{}, errRotationConfigurationInvalid
	}
	newCipher, err := provider.NewAPIKeyCipher(newSecret, newKeyID)
	if err != nil || oldKeyID == newKeyID {
		return rotationOptions{}, errRotationConfigurationInvalid
	}
	if *apply && getenv(providerKeyRotationConfirmEnv) != providerKeyRotationConfirmToken {
		return rotationOptions{}, errRotationConfirmationRequired
	}

	return rotationOptions{
		oldCipher: oldCipher,
		newCipher: newCipher,
		apply:     *apply,
	}, nil
}

func (deps commandDependencies) withDefaults() commandDependencies {
	if deps.loadConfig == nil {
		deps.loadConfig = config.Load
	}
	if deps.openDatabase == nil {
		deps.openDatabase = database.Open
	}
	if deps.closeDatabase == nil {
		deps.closeDatabase = database.Close
	}
	if deps.rotate == nil {
		deps.rotate = func(ctx context.Context, db *gorm.DB, oldCipher provider.APIKeyCipher, newCipher provider.APIKeyCipher, apply bool) (provider.APIKeyRotationSummary, error) {
			return provider.NewAPIKeyRotationService(db).Rotate(ctx, oldCipher, newCipher, apply)
		}
	}
	return deps
}
