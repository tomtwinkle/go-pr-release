package env

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func LookUpString(env string, required bool) (string, error) {
	if v, ok := os.LookupEnv(env); ok {
		return v, nil
	}
	if required {
		return "", fmt.Errorf("environment variable is not set to %s", env)
	}
	return "", nil
}

func LookUpStringSlice(env string, required bool, sep string) ([]string, error) {
	if v, err := LookUpString(env, required); err != nil {
		return nil, err
	} else if v == "" {
		return nil, nil
	} else {
		return strings.Split(v, sep), nil
	}
}

func LookUpInt(env string, required bool) (int, error) {
	if envValue, err := LookUpString(env, required); err != nil {
		return 0, err
	} else if envValue == "" {
		return 0, nil
	} else {
		if v, err := strconv.Atoi(envValue); err != nil {
			return 0, err
		} else {
			return v, nil
		}
	}
}

func LookUpBool(env string, required bool) (bool, error) {
	if envValue, err := LookUpString(env, required); err != nil {
		return false, err
	} else if envValue == "" {
		return false, nil
	} else {
		if v, err := strconv.ParseBool(envValue); err != nil {
			return false, err
		} else {
			return v, nil
		}
	}
}
