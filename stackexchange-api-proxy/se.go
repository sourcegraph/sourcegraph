package se

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sync"
	"time"
)

var (
	ErrURLNotAllowed  = errors.New("url is not allowed")
	ErrTimeoutLocking = errors.New("took too long to get a lock")
)

// Error wrap the original error and propagates an
// Op string to help understand where in the
// package they originated. This is modelled
// after the way things like url.Parse propagate
// their errors.
type Error struct {
	err error
	Op  string
}

func (e Error) Error() string {
	return fmt.Sprintf("StackExchange package error: %q (wraps %q)", e.Op, e.err)
}

type AllowList map[string]*regexp.Regexp

// DefaultAllowList
var DefaultAllowListPatterns = AllowList{
	"stackoverflow": regexp.MustCompile("^(|www.)stackoverflow.com$"),
}

// DefaultLockMechanismer implements an extremely naïve locker
// for any URL it returns the same mutex
var DefaultLockMechanismer = (&syncMutexLockMechanism{sm: sync.Mutex{}}).LockMechanism

type syncMutexLockMechanism struct {
	sm sync.Mutex
}

func (smlm *syncMutexLockMechanism) LockMechanism(_ url.URL) sync.Locker {
	return &smlm.sm
}

// Client encapsulates logic for speaking to a StackExchange
// compatible API (targeted API v2.2).
type Client struct {
	allowList       AllowList
	lockWaitTimeout time.Duration
	lockMechanismer LockMechanismer
}

// NewClient allows passing a variadic list of ClientOptionFns
// to configure a client based on the defaults.
func NewClient(optFns ...ClientOptionFn) (*Client, error) {

	var c = &Client{
		allowList:       DefaultAllowListPatterns,
		lockWaitTimeout: DefaultLockWaitTimeout,
		lockMechanismer: DefaultLockMechanismer,
	}

	for _, optFn := range optFns {
		optFn(c)
	}

	return c, nil
}

// IsAllowedURL takes a URL string and tries to parse it
// with url.Parse. Upon success the host part of the URL
// will be compared with an allow list.
//
// Malformed URLs return false without proporgating an
// error, callers who are not certian if they even have
// a valid URL are advised to use url.Parse and consult
// the url.error.Op field.
//
// The naive looping search may cause a problem if the
// allow list grows significantly, this should be instrumented
// if the allow list grows
func (c Client) IsAllowedURL(s string) (*url.Values, bool) {

	parsedURL, err := url.Parse(s)
	if err != nil {
		return nil, false
	}

	var anyMatch bool
	var matchedSite string
	for sn, re := range c.allowList {
		if match := re.MatchString(parsedURL.Hostname()); match {
			anyMatch = true
			matchedSite = sn
			break
		}
	}

	if anyMatch == false {
		return nil, false
	}

	return &url.Values{"site": []string{matchedSite}}, true
}

// FetchUpdate takes a a URL and examines it for StackExchange API
// compatibility, if the URL is in the allow list a request will
// be made to that URL, answers will ne fetched, code samples parsed
// out of the question, and answer markdowns.
func (c Client) FetchUpdate(ctx context.Context, s string) error {

	if _, allowed := c.IsAllowedURL(s); !allowed {
		return ErrURLNotAllowed
	}

	u, err := url.Parse(s)
	if err != nil {
		fmt.Printf("%#v %q", u, err)
		return Error{Op: "parse-url", err: err}
	}

	locker := c.lockMechanismer(*u)

	// wait for the lock in a goroutine, as if
	// it blocks, we'll block the calling goroutine
	// indefinitely.
	//
	// Use a context derived from the caller's context
	// with a timeout applied based on the client
	// configuration.
	//
	// We cancel the lockCtx on successful lock, and
	// this propagates as a "Cancelled" error on lockCtx.Err
	// which we can use later to know if we succeeded or failed.
	//
	// Cancellation in our case is desired.
	lockCtx, cancelFn := context.WithTimeout(ctx, c.lockWaitTimeout)
	go func(cancel context.CancelFunc, l sync.Locker) {
		l.Lock()
		cancel()
	}(cancelFn, locker)

	select {
	// the lock context yielded first
	case <-lockCtx.Done():
		if lockCtx.Err() == context.DeadlineExceeded {
			return ErrTimeoutLocking
		}
		defer locker.Unlock()

	// the given context (time constrained?) yielded
	// first
	// case <-ctx.Done():
	// 	fmt.Println("hit the client's timeout")
	// }

	return nil
}

// DefaultClient exposes a simple API that does not require
// extensive configuration to allow simple use of the package
// in cases such as pre-flighting a URL without configuring
// a fully-fledged client.
var DefaultClient, _ = NewClient()

// DefaultLockWaitTimeout is 5 seconds, after which
// we'll return an error in FetchUpdate
var DefaultLockWaitTimeout = 5 * time.Second

// IsAllowedURL is a simple function reference exposed
// to make the external API more pleasant to use in
// the common case.
var IsAllowedURL = DefaultClient.IsAllowedURL

// LockMechanismer takes a url.URL and returns a sync.Locker
// that is used to prevent concurrent requests to refresh that
// resource. A naîve implementation may simply pass back
// a shared sync.Mutex, the LockMechanismer is under no obligation
// to make use of finer-grained locks.
type LockMechanismer func(u url.URL) sync.Locker

// ClientOptionFn is a function to configure
// options on Client such as the LockMechanismer
// or timeouts, etc.
type ClientOptionFn func(c *Client) error

// SpecifyLockMechanism allows to set the lock mechanism
// a LockMechanism may optionally take a URL or similar
// for now the LockMechanism is only implemented
// as a simple sync.Mutex
func SpecifyLockMechanism(lm LockMechanismer) ClientOptionFn {
	// Should maybe check no locks are held before
	// allowing this to be overwritten?
	return func(c *Client) error {
		c.lockMechanismer = lm
		return nil
	}
}

// SpecifyAllowList specifies the AllowList for a Client {
func SpecifyAllowList(al AllowList) ClientOptionFn {
	return func(c *Client) error {
		c.allowList = al
		return nil
	}
}

// SpecifyLockWaitTimeout returns a ClientOptionsFn that interplays
// with the LockMechanismer and prevents the client waiting longer
// than the specified ttl to get a lock on a URL before returning
// an error.
func SpecifyLockWaitTimeout(ttl time.Duration) ClientOptionFn {
	return func(c *Client) error {
		c.lockWaitTimeout = ttl
		return nil
	}
}
