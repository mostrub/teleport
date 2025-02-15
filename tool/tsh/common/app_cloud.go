// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tlsca"
)

func defaultValue[t any]() t {
	var def t
	return def
}

// pickCloudApp will attempt to find an active cloud app, automatically logging the user to the selected application if possible.
func pickCloudApp[cloudApp any](cf *CLIConf, cloudFriendlyName string, matchRouteToApp func(tlsca.RouteToApp) bool, newCloudApp func(cf *CLIConf, profile *client.ProfileStatus, appRoute tlsca.RouteToApp) (cloudApp, error)) (cloudApp, error) {
	app, needLogin, err := pickActiveCloudApp[cloudApp](cf, cloudFriendlyName, matchRouteToApp, newCloudApp)
	if err != nil {
		if !needLogin {
			return defaultValue[cloudApp](), trace.Wrap(err)
		}
		log.WithError(err).Debugf("Failed to pick an active %v app, attempting to login into app %q", cloudFriendlyName, cf.AppName)
		quiet := cf.Quiet
		cf.Quiet = true
		errLogin := onAppLogin(cf)
		cf.Quiet = quiet
		if errLogin != nil {
			log.WithError(errLogin).Debugf("App login attempt failed")
			// combine errors
			return defaultValue[cloudApp](), trace.NewAggregate(err, errLogin)
		}
		// another attempt
		app, _, err = pickActiveCloudApp[cloudApp](cf, cloudFriendlyName, matchRouteToApp, newCloudApp)
		return app, trace.Wrap(err)
	}
	return app, nil
}

func pickActiveCloudApp[cloudApp any](cf *CLIConf, cloudFriendlyName string, matchRouteToApp func(tlsca.RouteToApp) bool, newCloudApp func(cf *CLIConf, profile *client.ProfileStatus, appRoute tlsca.RouteToApp) (cloudApp, error)) (cApp cloudApp, needLogin bool, err error) {
	profile, err := cf.ProfileStatus()
	if err != nil {
		return defaultValue[cloudApp](), false, trace.Wrap(err)
	}
	if len(profile.Apps) == 0 {
		if cf.AppName == "" {
			return defaultValue[cloudApp](), false, trace.NotFound("please login to %v app using 'tsh apps login' first", cloudFriendlyName)
		}
		return defaultValue[cloudApp](), true, trace.NotFound("please login to %v app using 'tsh apps login %v' first", cloudFriendlyName, cf.AppName)
	}
	name := cf.AppName
	if name != "" {
		app, err := findApp(profile.Apps, name)
		if err != nil {
			if trace.IsNotFound(err) {
				return defaultValue[cloudApp](), true, trace.NotFound("please login to %v app using 'tsh apps login %v' first", cloudFriendlyName, name)
			}
			return defaultValue[cloudApp](), false, trace.Wrap(err)
		}
		if !matchRouteToApp(*app) {
			return defaultValue[cloudApp](), false, trace.BadParameter(
				"selected app %q is not an %v application", name, cloudFriendlyName,
			)
		}

		cApp, err := newCloudApp(cf, profile, *app)
		return cApp, false, trace.Wrap(err)
	}

	filteredApps := filterApps(matchRouteToApp, profile.Apps)
	if len(filteredApps) == 0 {
		// no app name to use for attempted login.
		return defaultValue[cloudApp](), false, trace.NotFound("please login to %v App using 'tsh apps login' first", cloudFriendlyName)
	}
	if len(filteredApps) > 1 {
		names := strings.Join(getAppNames(filteredApps), ", ")
		return defaultValue[cloudApp](), false, trace.BadParameter(
			"multiple %v apps are available (%v), please specify one using --app CLI argument", cloudFriendlyName, names,
		)
	}
	cApp, err = newCloudApp(cf, profile, filteredApps[0])
	return cApp, false, trace.Wrap(err)
}

func filterApps(matchRouteToApp func(tlsca.RouteToApp) bool, apps []tlsca.RouteToApp) []tlsca.RouteToApp {
	var out []tlsca.RouteToApp
	for _, app := range apps {
		if matchRouteToApp(app) {
			out = append(out, app)
		}
	}
	return out
}

func getAppNames(apps []tlsca.RouteToApp) []string {
	var out []string
	for _, app := range apps {
		out = append(out, app.Name)
	}
	return out
}

func findApp(apps []tlsca.RouteToApp, name string) (*tlsca.RouteToApp, error) {
	for _, app := range apps {
		if app.Name == name {
			return &app, nil
		}
	}
	return nil, trace.NotFound("failed to find app with %q name", name)
}
