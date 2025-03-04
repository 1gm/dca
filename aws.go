package dca

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

const (
	ParamStorePrefix          = "awsssm"
	ParamStorePlaintextPrefix = "awsssm://"
	ParamStoreEncryptedPrefix = "awsssme://"
)

func HasAWSParamStorePrefix(val string) bool {
	return strings.HasPrefix(val, ParamStorePrefix)
}

func HasAWSParamStorePlaintextPrefix(key string) bool {
	return strings.HasPrefix(key, ParamStorePlaintextPrefix)
}

func HasAWSParamStoreEncryptedPrefix(key string) bool {
	return strings.HasPrefix(key, ParamStoreEncryptedPrefix)
}

func StripAWSParamStorePrefix(key string) string {
	if strings.HasPrefix(key, ParamStoreEncryptedPrefix) {
		return strings.TrimPrefix(key, ParamStoreEncryptedPrefix)
	}
	if strings.HasPrefix(key, ParamStorePlaintextPrefix) {
		return strings.TrimPrefix(key, ParamStorePlaintextPrefix)
	}
	return key
}

func GetAWSParamStoreValue(ctx context.Context, key string) (_ []byte, err error) {
	defer AddErr(&err, "dca.GetAWSParamStoreValue")
	if HasAWSParamStorePlaintextPrefix(key) {
		return GetAWSSParamStoreParameter(ctx, key)
	} else if HasAWSParamStoreEncryptedPrefix(key) {
		return GetAWSParamStoreEncryptedParameter(ctx, key)
	}
	return nil, fmt.Errorf("AWS Param Store key %s has an invalid prefix", key)
}

func GetAWSSParamStoreParameter(ctx context.Context, key string) ([]byte, error) {
	return getAWSParamStoreParameter(ctx, key, false)
}

func GetAWSParamStoreEncryptedParameter(ctx context.Context, key string) ([]byte, error) {
	return getAWSParamStoreParameter(ctx, key, true)
}

func getAWSParamStoreParameter(bgCtx context.Context, key string, encrypted bool) (_ []byte, err error) {
	defer WrapErr(&err, "dca.GetAWSParamStoreValue")

	ctx, cancel := context.WithTimeout(bgCtx, time.Second*5)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("error loading AWS configuration: %w", err)
	}

	ssmClient := ssm.NewFromConfig(cfg)

	strippedKey := StripAWSParamStorePrefix(key)

	out, err := ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &strippedKey,
		WithDecryption: &encrypted,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve parameter from ssm: %v", err)
	}
	return []byte(*out.Parameter.Value), nil
}
