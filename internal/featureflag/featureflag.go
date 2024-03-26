package featureflag

import (
	"encoding/binary"
	"hash/fnv"
	"time"
)

type FeatureFlag struct {
	Name string

	// A feature flag is one of the following types.
	// Exactly one of the following will be set.
	Bool    *FeatureFlagBool
	Rollout *FeatureFlagRollout

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// EvaluateForUser evaluates the feature flag for a userID.
func (f *FeatureFlag) EvaluateForUser(userID int32) bool {
	switch {
	case f.Bool != nil:
		return f.Bool.Value
	case f.Rollout != nil:
		return f.Rollout.Evaluate(f.Name, userID)
	}
	panic("one of Bool or Rollout must be set")
}

func hashUserAndFlag(userID int32, flagName string) uint32 {
	h := fnv.New32()
	binary.Write(h, binary.LittleEndian, userID)
	h.Write([]byte(flagName))
	return h.Sum32()
}

// EvaluateForAnonymousUser evaluates the feature flag for an anonymous user ID.
func (f *FeatureFlag) EvaluateForAnonymousUser(anonymousUID string) bool {
	switch {
	case f.Bool != nil:
		return f.Bool.Value
	case f.Rollout != nil:
		return hashAnonymousUserAndFlag(anonymousUID, f.Name)%10000 < uint32(f.Rollout.Rollout)
	}
	panic("one of Bool or Rollout must be set")
}

func hashAnonymousUserAndFlag(anonymousUID, flagName string) uint32 {
	h := fnv.New32()
	h.Write([]byte(anonymousUID))
	h.Write([]byte(flagName))
	return h.Sum32()
}

// EvaluateGlobal returns the evaluated feature flag for a global context (no user
// is associated with the request). If the flag is not evaluatable in the global context
// (i.e. the flag type is a rollout), then the second parameter will return false.
func (f *FeatureFlag) EvaluateGlobal() (res bool, ok bool) {
	switch {
	case f.Bool != nil:
		return f.Bool.Value, true
	}
	// ignore non-concrete feature flags since we have no active user
	return false, false
}

type FeatureFlagBool struct {
	Value bool
}

type FeatureFlagRollout struct {
	// Rollout is an integer between 0 and 10000, representing the percent of
	// users for which this feature flag will evaluate to 'true' in increments
	// of 0.01%
	Rollout int32
}

func (f *FeatureFlagRollout) Evaluate(flagName string, userID int32) bool {
	return hashUserAndFlag(userID, flagName)%10000 < uint32(f.Rollout)
}

type Override struct {
	UserID   *int32
	OrgID    *int32
	FlagName string
	Value    bool
}
