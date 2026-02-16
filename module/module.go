// Copyright 2013 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

/*
Package module provides functions for interacting with modules.

The appengine package contains functions that report the identity of the app,
including the module name.
*/
package module // import "google.golang.org/appengine/module"

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/golang/protobuf/proto"
	"google.golang.org/appengine"
	"google.golang.org/appengine/internal"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	pb "google.golang.org/appengine/internal/modules"

	admin "google.golang.org/api/appengine/v1"
)

func getProjectID() string {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	
	if (projectID == "") {
		appID := os.Getenv("GAE_APPLICATION")
		if appID == ""{
			appID = os.Getenv("APPLICATION_ID")
		}
		if i := strings.Index(appID, "~"); i != -1 {
			projectID = appID[i+1:]
		}
		// Strip domain prefix (e.g., "google.com:project-id" -> "project-id")
		if i := strings.Index(projectID, ":"); i != -1 {
			projectID = projectID[i+1:]
		}
		return projectID
	}
	
	return projectID
}

func getModuleorDefault() string {
	module := os.Getenv("GAE_SERVICE")
	if module == "" {
	    module = "default"
	}
	return module
}

// useAdminAPI checks if the Admin API implementation is enabled via environment variable.
func useAdminAPI() bool {
	return strings.ToLower(os.Getenv("MODULES_USE_ADMIN_API")) == "true"
}

// getService initializes the App Engine Admin API service.
func getAdminService(ctx context.Context, methodName string) (*admin.APIService, error) {
	userAgent := "appengine-modules-api-go-client/" + methodName
	svc, err := admin.NewService(ctx, option.WithUserAgent(userAgent))
	if err != nil {
		return nil, fmt.Errorf("module: could not create admin service: %v", err)
	}
	
	return svc, nil
}

// List returns the names of modules belonging to this application.
func List(c context.Context) ([]string, error) {
	if (!useAdminAPI()) {
		return ListLegacy(c)
	}
	projectID := getProjectID()
	svc, err := getAdminService(c, "get_modules")
	if err != nil {
		return nil, err
	}
	resp, err := svc.Apps.Services.List(projectID).Do()
	if err != nil {
		return nil, err
	}
	var modules []string
	for _, s := range resp.Services {
		modules = append(modules, s.Id)
	}
	return modules, nil
}

func ListLegacy(c context.Context) ([]string, error) {
	req := &pb.GetModulesRequest{}
	res := &pb.GetModulesResponse{}
	err := internal.Call(c, "modules", "GetModules", req, res)
	return res.Module, err
}

// NumInstances returns the number of instances of the given module/version.
// If either argument is the empty string it means the default.
func NumInstances(c context.Context, module, version string) (int, error) {
	if (!useAdminAPI()) {
		return NumInstancesLegacy(c, module, version)
	}
	if module == "" {
		module = getModuleorDefault()
	}
	if version == "" {
		version = appengine.VersionID(c)
	}
	projectID := getProjectID()
	svc, err := getAdminService(c, "get_num_instances")
	if err != nil {
		return 0, err
	}
	v, err := svc.Apps.Services.Versions.Get(projectID, module, version).Do()
	if err != nil {
		return 0, err
	}
	if v.ManualScaling != nil {
		return int(v.ManualScaling.Instances), nil
	}
	return 0, fmt.Errorf("module: version %s is not using manual scaling", version)
}

func NumInstancesLegacy(c context.Context, module, version string) (int, error) {
	req := &pb.GetNumInstancesRequest{}
	if module != "" {
		req.Module = &module
	}
	if version != "" {
		req.Version = &version
	}
	res := &pb.GetNumInstancesResponse{}

	if err := internal.Call(c, "modules", "GetNumInstances", req, res); err != nil {
		return 0, err
	}
	return int(*res.Instances), nil
}

// SetNumInstances sets the number of instances of the given module.version to the
// specified value. If either module or version are the empty string it means the
// default.
func SetNumInstances(c context.Context, module, version string, instances int) error {
	if (!useAdminAPI()) {
		return SetNumInstancesLegacy(c, module, version, instances)
	}
	if module == "" {
		module = getModuleorDefault()
	}
	if version == "" {
		version = appengine.VersionID(c)
	}
	projectID := getProjectID()
	svc, err1 := getAdminService(c, "set_num_instances")
	if err1 != nil {
		return err1
	}
	update := &admin.Version{
		ManualScaling: &admin.ManualScaling{
			Instances: int64(instances),
		},
	}
	_, err2 := svc.Apps.Services.Versions.Patch(projectID, module, version, update).
		UpdateMask("manualScaling.instances").Do()
	return err2
}

func SetNumInstancesLegacy(c context.Context, module, version string, instances int) error {
	req := &pb.SetNumInstancesRequest{}
	if module != "" {
		req.Module = &module
	}
	if version != "" {
		req.Version = &version
	}
	req.Instances = proto.Int64(int64(instances))
	res := &pb.SetNumInstancesResponse{}
	return internal.Call(c, "modules", "SetNumInstances", req, res)
}

// Versions returns the names of the versions that belong to the specified module.
// If module is the empty string, it means the default module.
func Versions(c context.Context, module string) ([]string, error) {
	if (!useAdminAPI()) {
		return VersionsLegacy(c, module)
	}
	if module == "" {
		module = getModuleorDefault()
	}
	projectID := getProjectID()
	svc, err := getAdminService(c, "get_versions")
	if err != nil {
		return nil, err
	}
	resp, err := svc.Apps.Services.Versions.List(projectID, module).Do()
	if err != nil {
		return nil, err
	}
	var versions []string
	for _, v := range resp.Versions {
		versions = append(versions, v.Id)
	}
	return versions, nil
}

func VersionsLegacy(c context.Context, module string) ([]string, error) {
	req := &pb.GetVersionsRequest{}
	if module != "" {
		req.Module = &module
	}
	res := &pb.GetVersionsResponse{}
	err := internal.Call(c, "modules", "GetVersions", req, res)
	return res.GetVersion(), err
}

// DefaultVersion returns the default version of the specified module.
// If module is the empty string, it means the default module.
func DefaultVersion(c context.Context, module string) (string, error) {
	if (!useAdminAPI()) {
		return DefaultVersionLegacy(c, module)
	}
	if module == "" {
		module = getModuleorDefault()
	}
	projectID := getProjectID()
	svc, err := getAdminService(c, "get_default_version")
	if err != nil {
		return "", err
	}
	service, err := svc.Apps.Services.Get(projectID, module).Context(c).Do()
	if err != nil {
		if gErr, ok := err.(*googleapi.Error); ok && gErr.Code == 404 {
			return "", fmt.Errorf("module: Module '%s' not found", module)
		}
		return "", err
	}

	// Logic to determine default version from traffic split allocations
	var retVersion string
	maxAlloc := -1.0

	if service.Split != nil && service.Split.Allocations != nil {
		allocations := service.Split.Allocations // map[string]float64

		for version, allocation := range allocations {
			// If a version has 100% traffic, it is the default
			if allocation == 1.0 {
				retVersion = version
				break
			}

			// Find version with maximum allocation
			if allocation > maxAlloc {
				retVersion = version
				maxAlloc = allocation
			} else if allocation == maxAlloc {
				// Tie-breaker: lexicographically smaller version name
				if version < retVersion {
					retVersion = version
				}
			}
		}
	}

	// Equivalent to if retVersion is None: raise InvalidVersionError
	if retVersion == "" {
		return "", fmt.Errorf("module: could not determine default version for module '%s'", module)
	}

	return retVersion, nil
}

func DefaultVersionLegacy(c context.Context, module string) (string, error) {
	req := &pb.GetDefaultVersionRequest{}
	if module != "" {
		req.Module = &module
	}
	res := &pb.GetDefaultVersionResponse{}
	err := internal.Call(c, "modules", "GetDefaultVersion", req, res)
	return res.GetVersion(), err
}

// Start starts the specified version of the specified module.
// If either module or version are the empty string, it means the default.
func Start(c context.Context, module, version string) error {
	if (!useAdminAPI()) {
		return StartLegacy(c, module, version)
	}
	return setServingStatus(c, module, version, "SERVING")
}

func StartLegacy(c context.Context, module, version string) error {
	req := &pb.StartModuleRequest{}
	if module != "" {
		req.Module = &module
	}
	if version != "" {
		req.Version = &version
	}
	res := &pb.StartModuleResponse{}
	return internal.Call(c, "modules", "StartModule", req, res)
}

// Stop stops the specified version of the specified module.
// If either module or version are the empty string, it means the default.
func Stop(c context.Context, module, version string) error {
	if (!useAdminAPI()) {
		return StopLegacy(c, module, version)
	}
	return setServingStatus(c, module, version, "STOPPED")
}

func StopLegacy(c context.Context, module, version string) error {
	req := &pb.StopModuleRequest{}
	if module != "" {
		req.Module = &module
	}
	if version != "" {
		req.Version = &version
	}
	res := &pb.StopModuleResponse{}
	return internal.Call(c, "modules", "StopModule", req, res)
}

func setServingStatus(c context.Context, module, version, status string) error {
	projectID := getProjectID()
	methodName := ""
	if status == "SERVING" {
		methodName = "start_version"
	} else if status == "STOPPED" {
		methodName = "stop_version"
	}
	svc, err := getAdminService(c, methodName)
	if err != nil {
		return err
	}
	if module == "" {
		module = getModuleorDefault()
	}
	if version == "" {
		version = appengine.VersionID(c)
	}
	update := &admin.Version{
		ServingStatus: status,
	}
	_, err = svc.Apps.Services.Versions.Patch(projectID, module, version, update).
		UpdateMask("servingStatus").Do()
	return err
}
