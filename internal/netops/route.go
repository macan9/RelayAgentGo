package netops

import (
	"context"
	"fmt"
	"strconv"
)

type Route struct {
	Dst    string
	Via    string
	Dev    string
	Metric int
}

type RouteManager struct {
	runner Runner
}

func NewRouteManager(runner Runner) *RouteManager {
	return &RouteManager{runner: runner}
}

func (manager *RouteManager) List(ctx context.Context) (Result, error) {
	return manager.runner.Run(ctx, Command{
		Name: "ip",
		Args: []string{"route", "show"},
	})
}

func (manager *RouteManager) Replace(ctx context.Context, route Route) (Result, error) {
	args, err := routeArgs("replace", route)
	if err != nil {
		return Result{}, err
	}
	return manager.runner.Run(ctx, Command{Name: "ip", Args: args})
}

func (manager *RouteManager) Delete(ctx context.Context, route Route) (Result, error) {
	args, err := routeArgs("del", route)
	if err != nil {
		return Result{}, err
	}
	return manager.runner.Run(ctx, Command{Name: "ip", Args: args})
}

func routeArgs(action string, route Route) ([]string, error) {
	if route.Dst == "" {
		return nil, fmt.Errorf("route dst is required")
	}

	args := []string{"route", action, route.Dst}
	if route.Via != "" {
		args = append(args, "via", route.Via)
	}
	if route.Dev != "" {
		args = append(args, "dev", route.Dev)
	}
	if route.Metric > 0 {
		args = append(args, "metric", strconv.Itoa(route.Metric))
	}

	return args, nil
}
