// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

package ipn

import (
	"testing"
	"time"
)

func TestServicePrefsEquals(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		a, b ServicePrefs
		want bool
	}{
		{"both nil", nil, nil, true},
		{"both empty", ServicePrefs{}, ServicePrefs{}, true},
		{"nil vs empty", nil, ServicePrefs{}, true},
		{
			"same key + same value",
			ServicePrefs{"svc:db:5432": {Client: "psql", LastUsed: now}},
			ServicePrefs{"svc:db:5432": {Client: "psql", LastUsed: now}},
			true,
		},
		{
			"different client",
			ServicePrefs{"svc:db:5432": {Client: "psql"}},
			ServicePrefs{"svc:db:5432": {Client: "pgcli"}},
			false,
		},
		{
			"different key",
			ServicePrefs{"svc:db:5432": {Client: "psql"}},
			ServicePrefs{"svc:other:5432": {Client: "psql"}},
			false,
		},
		{
			"different length",
			ServicePrefs{"svc:db:5432": {Client: "psql"}},
			ServicePrefs{
				"svc:db:5432":  {Client: "psql"},
				"svc:web:8080": {},
			},
			false,
		},
		{
			"all fields match",
			ServicePrefs{"svc:ssh:22": {Client: "terminal", Username: "rollie", DatabaseName: "", LastUsed: now}},
			ServicePrefs{"svc:ssh:22": {Client: "terminal", Username: "rollie", DatabaseName: "", LastUsed: now}},
			true,
		},
		{
			"different username",
			ServicePrefs{"svc:ssh:22": {Client: "terminal", Username: "rollie"}},
			ServicePrefs{"svc:ssh:22": {Client: "terminal", Username: "other"}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equals(tt.b); got != tt.want {
				t.Errorf("(%v).Equals(%v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
			if got := tt.b.Equals(tt.a); got != tt.want {
				t.Errorf("(%v).Equals(%v) = %v, want %v (reverse)", tt.b, tt.a, got, tt.want)
			}
		})
	}
}

func TestServicePrefsClone(t *testing.T) {
	now := time.Now()
	src := ServicePrefs{
		"svc:db:5432": {Client: "psql", DatabaseName: "prod", LastUsed: now},
		"svc:ssh:22":  {Client: "terminal", Username: "rollie", LastUsed: now},
	}
	dst := src.Clone()
	if !src.Equals(dst) {
		t.Fatalf("Clone result not equal to source")
	}
	// Mutating dst must not affect src.
	dst["svc:db:5432"] = ServicePref{Client: "pgcli"}
	if src["svc:db:5432"].Client != "psql" {
		t.Errorf("mutating clone leaked into source: %v", src["svc:db:5432"])
	}
	// Nil clone returns nil.
	if got := ServicePrefs(nil).Clone(); got != nil {
		t.Errorf("(nil).Clone() = %v, want nil", got)
	}
}
