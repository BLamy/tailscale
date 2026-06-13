// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

// Package routecheck registers support for RouteCheck,
// which checks the reachability of overlapping routers.
//
// When there are multiple network paths to an IP address, it is being routed by
// overlapping routers. The client uses reachability to pick between those
// paths: either sticking with an active WireGuard session or choosing from the
// peers that it has determined it can reach. It doesn’t need reachability for
// IP addresses that have only one network path, since it can naively attempt to
// establish a WireGuard session.
package routecheck

import (
	"context"
	"errors"
	"fmt"

	"tailscale.com/ipn/ipnext"
	"tailscale.com/net/routecheck"
	"tailscale.com/tailcfg"
	"tailscale.com/types/logger"
)

// FeatureName is the name of the feature implemented by this package.
// It is also the [extension] name and the log prefix.
const featureName = "routecheck"

func init() {
	ipnext.RegisterExtension(featureName, func(logf logger.Logf, b ipnext.SafeBackend) (ipnext.Extension, error) {
		return &Extension{
			logf:    logger.WithPrefix(logf, featureName+": "),
			backend: b,
		}, nil
	})
}

// Extension implements the [ipnext.Extension] interface.
type Extension struct {
	Client *routecheck.Client

	logf    logger.Logf
	backend ipnext.SafeBackend
	nb      nodeBackender
	nm      routecheck.NetMapper
	routers *RouterTracker
}

var _ ipnext.Extension = new(Extension)

// Name implements the [ipnext.Extension.Name] interface method.
func (e *Extension) Name() string {
	return featureName
}

// Init implements the [ipnext.Extension.Init] interface method.
func (e *Extension) Init(h ipnext.Host) error {
	if routecheck.DebugForceClientSideReachabilityRoutecheck().EqualBool(false) {
		return ipnext.SkipExtension
	}

	ctx := context.Background()

	e.nb = nodeBackender{h}

	nm, ok := e.backend.(routecheck.NetMapper)
	if !ok {
		return fmt.Errorf("backend %T does not implement routecheck.NetMapper", e.backend)
	}
	e.nm = nm

	ipnbus, ok := e.backend.(ipnext.NotifyWatcher)
	if !ok {
		return fmt.Errorf("backend %T does not implement ipnext.NotifyWatcher", e.backend)
	}

	pinger := e.backend.Sys().Engine.Get()

	c, err := routecheck.NewClient(e.logf, e.nb, e.nm, pinger)
	if err != nil {
		return err
	}
	e.Client = c

	e.routers = TrackRouters(ctx, e.logf, ipnbus)
	e.routers.OnNetMapAvailable = e.Client.NotifyNetMapAvailable
	e.routers.OnRoutersChange = e.onRoutersChange

	h.Hooks().OnSelfChange.Add(e.onSelfChange)

	// Unlike a cold start, starting with a cached netmap
	// may have pre-loaded a valid NodeBackend.Self,
	// so an initial OnSelfChange won’t fire and we have to do it ourselves.
	// If the cached netmap was running the watcher goroutine, this will restart it.
	if self := e.nb.NodeBackend().Self(); self.Valid() {
		e.onSelfChange(self)
	}

	return nil
}

// Shutdown implements the [ipnext.Extension.Shutdown] interface method.
func (e *Extension) Shutdown() error {
	e.routers.Close()
	return e.Client.Close()
}

func (e *Extension) needsRefresh() {
	if !routecheck.IsEnabled(e.nb.NodeBackend().Self()) {
		return
	}
	// TODO(sfllaw): e.Client.NeedsRefresh()
}

func (e *Extension) onRoutersChange(added, modified, removed []tailcfg.NodeID) {
	// TODO(sfllaw): This refresh could be incremental,
	// based on the added, modified, and removed nodes.
	e.needsRefresh()
}

func (e *Extension) onSelfChange(self tailcfg.NodeView) {
	// onSelfChange must never block, because it’s called from
	// [ipnlocal.LocalBackend.SetControlClientStatus], which locks
	// LocalBackend.mu. This lock is also acquired when unwinding
	// [ipnlocal.LocalBackend.WatchNotificationsAs] which is what
	// [RouterTracker.stopWatcherLocked] is waiting for.
	go func() {
		if _, _, err := e.routers.StartStopWatcher(self); err != nil {
			if !errors.Is(err, routecheckNotEnabledErr) {
				e.logf("error tracking routers: %v", err)
			}
			return // can be started by toggling the nodeattr
		}
		e.needsRefresh()
	}()
}
