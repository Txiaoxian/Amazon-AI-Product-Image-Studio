package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"gorm.io/gorm"
)

func TestRotationOptionsFromEnvDefaultsToDryRun(t *testing.T) {
	options, err := rotationOptionsFromEnv(nil, lookupRotationEnv(nil))
	if err != nil {
		t.Fatalf("rotationOptionsFromEnv returned error: %v", err)
	}
	if options.apply {
		t.Fatal("rotation options enabled apply without --apply")
	}
}

func TestRotationOptionsFromEnvApplyRequiresExactConfirmation(t *testing.T) {
	for _, test := range []struct {
		name    string
		confirm string
		wantErr bool
	}{
		{name: "missing", wantErr: true},
		{name: "wrong", confirm: "yes", wantErr: true},
		{name: "exact", confirm: providerKeyRotationConfirmToken, wantErr: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := map[string]string{providerKeyRotationConfirmEnv: test.confirm}
			options, err := rotationOptionsFromEnv([]string{"--apply"}, lookupRotationEnv(env))
			if test.wantErr {
				if !errors.Is(err, errRotationConfirmationRequired) {
					t.Fatalf("rotationOptionsFromEnv error = %v, want errRotationConfirmationRequired", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("rotationOptionsFromEnv returned error: %v", err)
			}
			if !options.apply {
				t.Fatal("rotation options did not enable apply")
			}
		})
	}
}

func TestRotationOptionsFromEnvRejectsArgumentsAndInvalidEnvironmentWithoutLeakingValues(t *testing.T) {
	const sensitiveMarker = "sensitive-marker-that-must-not-leak-123456789"

	for _, test := range []struct {
		name string
		args []string
		env  map[string]string
	}{
		{name: "unknown flag", args: []string{"--secret=" + sensitiveMarker}},
		{name: "positional argument", args: []string{sensitiveMarker}},
		{name: "missing old secret", env: map[string]string{providerKeyRotationOldSecretEnv: ""}},
		{name: "same key id", env: map[string]string{providerKeyRotationNewKeyIDEnv: rotationTestOldKeyID}},
		{name: "invalid key id", env: map[string]string{providerKeyRotationNewKeyIDEnv: sensitiveMarker + ":invalid"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := rotationOptionsFromEnv(test.args, lookupRotationEnv(test.env))
			if !errors.Is(err, errRotationConfigurationInvalid) {
				t.Fatalf("rotationOptionsFromEnv error = %v, want errRotationConfigurationInvalid", err)
			}
			if strings.Contains(err.Error(), sensitiveMarker) {
				t.Fatalf("rotationOptionsFromEnv error leaked sensitive marker: %v", err)
			}
		})
	}
}

func TestRunOutputsOnlySanitizedSummary(t *testing.T) {
	const sensitiveMarker = "sensitive-marker-that-must-not-leak-123456789"
	var output bytes.Buffer
	db := &gorm.DB{}

	err := run(context.Background(), nil, lookupRotationEnv(map[string]string{
		providerKeyRotationOldSecretEnv: sensitiveMarker + "-old",
		providerKeyRotationNewSecretEnv: sensitiveMarker + "-new",
	}), &output, commandDependencies{
		loadConfig: func() (config.Config, error) {
			return config.Config{Database: config.DatabaseConfig{ConnectTimeout: time.Second}}, nil
		},
		openDatabase: func(context.Context, config.DatabaseConfig) (*gorm.DB, error) {
			return db, nil
		},
		closeDatabase: func(*gorm.DB) error {
			return nil
		},
		rotate: func(_ context.Context, gotDB *gorm.DB, _ provider.APIKeyCipher, _ provider.APIKeyCipher, apply bool) (provider.APIKeyRotationSummary, error) {
			if gotDB != db {
				t.Fatal("run passed unexpected database")
			}
			if apply {
				t.Fatal("run enabled apply by default")
			}
			return provider.APIKeyRotationSummary{ProviderCount: 3, Applied: false}, nil
		},
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if got := output.String(); got != "provider key rotation mode=dry-run providers=3 result=success\n" {
		t.Fatalf("output = %q", got)
	}
	if strings.Contains(output.String(), sensitiveMarker) {
		t.Fatalf("output leaked sensitive marker: %q", output.String())
	}
}

func TestRunRedactsDependencyErrors(t *testing.T) {
	const sensitiveMarker = "sensitive-marker-that-must-not-leak"
	db := &gorm.DB{}

	for _, test := range []struct {
		name string
		deps commandDependencies
	}{
		{
			name: "load config",
			deps: commandDependencies{
				loadConfig: func() (config.Config, error) {
					return config.Config{}, errors.New(sensitiveMarker)
				},
			},
		},
		{
			name: "open database",
			deps: commandDependencies{
				loadConfig: func() (config.Config, error) {
					return config.Config{Database: config.DatabaseConfig{ConnectTimeout: time.Second}}, nil
				},
				openDatabase: func(context.Context, config.DatabaseConfig) (*gorm.DB, error) {
					return nil, errors.New(sensitiveMarker)
				},
			},
		},
		{
			name: "rotate",
			deps: commandDependencies{
				loadConfig: func() (config.Config, error) {
					return config.Config{Database: config.DatabaseConfig{ConnectTimeout: time.Second}}, nil
				},
				openDatabase: func(context.Context, config.DatabaseConfig) (*gorm.DB, error) {
					return db, nil
				},
				closeDatabase: func(*gorm.DB) error {
					return nil
				},
				rotate: func(context.Context, *gorm.DB, provider.APIKeyCipher, provider.APIKeyCipher, bool) (provider.APIKeyRotationSummary, error) {
					return provider.APIKeyRotationSummary{}, errors.New(sensitiveMarker)
				},
			},
		},
		{
			name: "close database",
			deps: commandDependencies{
				loadConfig: func() (config.Config, error) {
					return config.Config{Database: config.DatabaseConfig{ConnectTimeout: time.Second}}, nil
				},
				openDatabase: func(context.Context, config.DatabaseConfig) (*gorm.DB, error) {
					return db, nil
				},
				closeDatabase: func(*gorm.DB) error {
					return errors.New(sensitiveMarker)
				},
				rotate: func(context.Context, *gorm.DB, provider.APIKeyCipher, provider.APIKeyCipher, bool) (provider.APIKeyRotationSummary, error) {
					return provider.APIKeyRotationSummary{}, nil
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := run(context.Background(), nil, lookupRotationEnv(nil), &output, test.deps)
			if err == nil {
				t.Fatal("run returned nil error")
			}
			if strings.Contains(err.Error(), sensitiveMarker) || strings.Contains(output.String(), sensitiveMarker) {
				t.Fatalf("run leaked sensitive marker: err=%v output=%q", err, output.String())
			}
		})
	}
}

const (
	rotationTestOldKeyID = "rotation-key-v1"
	rotationTestNewKeyID = "rotation-key-v2"
)

func lookupRotationEnv(overrides map[string]string) func(string) string {
	env := map[string]string{
		providerKeyRotationOldSecretEnv: "0123456789abcdef0123456789abcdef",
		providerKeyRotationOldKeyIDEnv:  rotationTestOldKeyID,
		providerKeyRotationNewSecretEnv: "abcdef0123456789abcdef0123456789",
		providerKeyRotationNewKeyIDEnv:  rotationTestNewKeyID,
	}
	for key, value := range overrides {
		env[key] = value
	}
	return func(key string) string {
		return env[key]
	}
}
