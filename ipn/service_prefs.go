// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

package ipn

import (
	"maps"
	"time"
)

// ServicePrefs maps a service-action key to the user's saved preferences for that action.
// Keys are formatted as "<serviceName>:<port>" (e.g. "svc:my-db:5432"), matching the format
// the Tailscale macOS and Windows clients use to identify a service action.
//
// "Recently used" ordering is derived at query time by sorting entries with non-zero
// ServicePref.LastUsed descending. We don't store an ordered list. Same approach as
// xcode/IPN/macOS/Helpers/Services/ServicePreferences.swift's recentlyUsed(from:limit:).
type ServicePrefs map[string]ServicePref

// ServicePref captures the saved preferences for one service action.
type ServicePref struct {
	// Client is the saved client identifier the user picked last time. Interpretation is
	// service-type specific (e.g. "terminal"/"putty" for SSH, "dbeaver"/"psql" for database).
	// Empty means no client has been saved for this action.
	Client string

	// Username is the saved login name (SSH, RDP). Empty means none saved.
	Username string

	// DatabaseName is the saved DB name (database service types). Empty means none saved.
	DatabaseName string

	// LastUsed is when this service action was last launched. Zero means never.
	LastUsed time.Time
}

// Equals reports whether sp deep-equals other.
func (sp ServicePrefs) Equals(other ServicePrefs) bool {
	if len(sp) != len(other) {
		return false
	}
	for k, v := range sp {
		ov, ok := other[k]
		if !ok || v != ov {
			return false
		}
	}
	return true
}

// Clone returns a shallow copy of sp. ServicePref is a value type, so the entries are
// independent of the source map.
func (sp ServicePrefs) Clone() ServicePrefs {
	if sp == nil {
		return nil
	}
	return maps.Clone(sp)
}
