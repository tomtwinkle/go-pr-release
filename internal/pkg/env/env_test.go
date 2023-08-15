package env_test

import (
	"strings"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/stretchr/testify/assert"

	"github.com/tomtwinkle/go-pr-release/internal/pkg/env"
)

func TestLookUpString(t *testing.T) {
	type Param struct {
		Key      string
		Required bool
	}
	type Want struct {
		Val string
		Err bool
	}
	tests := map[string]struct {
		SetEnv func(t *testing.T) (Param, Want)
	}{
		"required:key exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := faker.Sentence()
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: val,
						Err: false,
					}
			},
		},
		"required:key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := faker.Sentence()
				t.Setenv(key, val)
				return Param{
						Key:      "NOT_EXIST",
						Required: true,
					}, Want{
						Err: true,
					}
			},
		},
		"not required:key exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := faker.Sentence()
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: false,
					}, Want{
						Val: val,
						Err: false,
					}
			},
		},
		"not required:key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := faker.Sentence()
				t.Setenv(key, val)
				return Param{
						Key:      "NOT_EXIST",
						Required: false,
					}, Want{
						Val: "",
						Err: false,
					}
			},
		},
	}

	for n, v := range tests {
		name := n
		tt := v
		t.Run(name, func(t *testing.T) {
			p, want := tt.SetEnv(t)
			got, err := env.LookUpString(p.Key, p.Required)
			if want.Err {
				assert.Error(t, err)
				return
			}
			if assert.NoError(t, err) {
				assert.Equal(t, want.Val, got)
			}
		})
	}
}

func TestLookUpStringSlice(t *testing.T) {
	type Param struct {
		Key      string
		Required bool
		Sep      string
	}
	type Want struct {
		Val []string
		Err bool
	}
	tests := map[string]struct {
		SetEnv func(t *testing.T) (Param, Want)
	}{
		"required:key exists:sep=,": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := []string{faker.Sentence(), faker.Sentence()}
				t.Setenv(key, strings.Join(val, ","))
				return Param{
						Key:      key,
						Required: true,
						Sep:      ",",
					}, Want{
						Val: val,
						Err: false,
					}
			},
		},
		"required:key exists:sep=|": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := []string{faker.Sentence(), faker.Sentence()}
				t.Setenv(key, strings.Join(val, "|"))
				return Param{
						Key:      key,
						Required: true,
						Sep:      "|",
					}, Want{
						Val: val,
						Err: false,
					}
			},
		},
		"required:key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				return Param{
						Key:      "NOT_EXIST",
						Required: true,
						Sep:      ",",
					}, Want{
						Err: true,
					}
			},
		},
		"not required:key exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := []string{faker.Sentence(), faker.Sentence()}
				t.Setenv(key, strings.Join(val, ","))
				return Param{
						Key:      key,
						Required: false,
						Sep:      ",",
					}, Want{
						Val: val,
						Err: false,
					}
			},
		},
		"not required:key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				return Param{
						Key:      "NOT_EXIST",
						Required: false,
						Sep:      ",",
					}, Want{
						Val: nil,
						Err: false,
					}
			},
		},
	}

	for n, v := range tests {
		name := n
		tt := v
		t.Run(name, func(t *testing.T) {
			p, want := tt.SetEnv(t)
			got, err := env.LookUpStringSlice(p.Key, p.Required, p.Sep)
			if want.Err {
				assert.Error(t, err)
				return
			}
			if assert.NoError(t, err) {
				assert.Equal(t, want.Val, got)
			}
		})
	}
}

func TestLookUpInt(t *testing.T) {
	type Param struct {
		Key      string
		Required bool
	}
	type Want struct {
		Val int
		Err bool
	}
	tests := map[string]struct {
		SetEnv func(t *testing.T) (Param, Want)
	}{
		"required:key exists:int": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "1"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: 1,
						Err: false,
					}
			},
		},
		"required:key exists:int = 0": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "0"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: 0,
						Err: false,
					}
			},
		},
		"required:key exists:not int": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "hoge"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Err: true,
					}
			},
		},
		"key exists:not int": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "hoge"
				t.Setenv(key, val)
				return Param{
						Key: key,
					}, Want{
						Err: true,
					}
			},
		},
		"required:key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				return Param{
						Key:      "NOT_EXIST",
						Required: true,
					}, Want{
						Err: true,
					}
			},
		},
		"key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				return Param{
						Key: "NOT_EXIST",
					}, Want{
						Val: 0,
						Err: false,
					}
			},
		},
	}

	for n, v := range tests {
		name := n
		tt := v
		t.Run(name, func(t *testing.T) {
			p, want := tt.SetEnv(t)
			got, err := env.LookUpInt(p.Key, p.Required)
			if want.Err {
				assert.Error(t, err)
				return
			}
			if assert.NoError(t, err) {
				assert.Equal(t, want.Val, got)
			}
		})
	}
}

func TestLookUpBool(t *testing.T) {
	type Param struct {
		Key      string
		Required bool
	}
	type Want struct {
		Val bool
		Err bool
	}
	tests := map[string]struct {
		SetEnv func(t *testing.T) (Param, Want)
	}{
		"required:key exists:true": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "true"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: true,
						Err: false,
					}
			},
		},
		"required:key exists:false": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "false"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: false,
						Err: false,
					}
			},
		},
		"required:key exists:1": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "1"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: true,
						Err: false,
					}
			},
		},
		"required:key exists:0": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "0"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: false,
						Err: false,
					}
			},
		},
		"required:key exists:not bool": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "hoge"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Err: true,
					}
			},
		},
		"key exists:not bool": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "hoge"
				t.Setenv(key, val)
				return Param{
						Key: key,
					}, Want{
						Err: true,
					}
			},
		},
		"required:key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				return Param{
						Key:      "NOT_EXIST",
						Required: true,
					}, Want{
						Err: true,
					}
			},
		},
		"key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				return Param{
						Key: "NOT_EXIST",
					}, Want{
						Val: false,
						Err: false,
					}
			},
		},
	}

	for n, v := range tests {
		name := n
		tt := v
		t.Run(name, func(t *testing.T) {
			p, want := tt.SetEnv(t)
			got, err := env.LookUpBool(p.Key, p.Required)
			if want.Err {
				assert.Error(t, err)
				return
			}
			if assert.NoError(t, err) {
				assert.Equal(t, want.Val, got)
			}
		})
	}
}
