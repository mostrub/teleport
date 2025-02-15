/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func FuzzParseRefs(f *testing.F) {
	// seeds from unit test examples
	f.Add("lock")
	f.Add("integration")
	f.Add("integration/00124f1e-d70e-413e-9b20-9b2d4c97e10c")
	f.Add("integration/unknown")
	f.Add("integration/myawsint")
	f.Add("app")
	f.Add("app/appB")
	f.Add("db_server")
	f.Add("db_server/example")
	f.Add("db")
	f.Add("db/example")
	f.Add("db_service")
	f.Add("db_service/7af76d49-b747-4bc1-b43d-c6dd457c229e")
	f.Add("db_service/unknown")
	// other seeds
	f.Add("foo,bar")
	f.Add("foo\\,bar/foobar")

	f.Fuzz(func(t *testing.T, refs string) {
		require.NotPanics(t, func() {
			ParseRefs(refs)
		})
	})
}

func FuzzParserEvalBoolPredicate(f *testing.F) {
	// seeds from unit tests
	f.Add("name == \"4a6t1q1zcsq97q\"")
	f.Add("labels.env == \"test\"")
	f.Add("contains(reviewer.roles,\"dev\")")
	f.Add("!contains(reviewer.traits[\"teams\"],\"staging-admin\")")
	f.Add("equals(request.reason,review.reason)")
	f.Add("contains(reviewer.roles, \"admin\")")
	f.Add("equals(fully.fake.path,\"should-fail\")")
	f.Add("fakefunc(reviewer.roles,\"some-role\")")
	f.Add("equals(\"too\",\"many\",\"params\")")
	f.Add("contains(\"missing-param\")")
	f.Add("&& missing-left")
	f.Add("labels.env.toomanyfield")
	f.Add("exists(labels.undefined)")
	f.Add("name.toomanyfield")
	f.Add("!name")
	f.Add("name ==")
	f.Add("equals(labels[\"env\"], \"wrong-value\")")
	f.Add("name ||")
	f.Add("&&")
	f.Add("||")
	f.Add("|")
	f.Add("&")
	f.Add("!")
	f.Add(".")
	f.Add("!exists(labels.env)")
	f.Add("name &&")
	f.Add("name &")
	f.Add("name |")
	f.Add("search(\"mac\", \"not-found\")")
	f.Add("hasPrefix(name, \"x\")")
	f.Add("search(\"mac\")")
	f.Add("equals()")
	f.Add("exists()")
	f.Add("search(1,2)")
	f.Add("\"just-string\"")
	f.Add("hasPrefix(1, 2)")
	f.Add("hasPrefix(name, \"too\", \"many\")")
	f.Add("hasPrefix(name, 1)")
	f.Add("search()")
	f.Add("resource.metadata.labels[\"env\"] == \"prod\"")
	f.Add("(exists(labels.env) || exists(labels.os)) && labels.os != \"mac\"")
	f.Add("search(\"does\", \"not\", \"exist\") || resource.spec.addr == \"_\" || labels.version == \"v8\"")

	f.Fuzz(func(t *testing.T, expr string) {
		resource, err := types.NewServerWithLabels("test-name", types.KindNode, types.ServerSpecV2{
			Hostname: "test-hostname",
			Addr:     "test-addr",
			CmdLabels: map[string]types.CommandLabelV2{
				"version": {
					Result: "v8",
				},
			},
		}, map[string]string{
			"env": "prod",
			"os":  "mac",
		})
		require.NoError(t, err)

		parser, err := NewResourceParser(resource)
		require.NoError(t, err)

		require.NotPanics(t, func() {
			parser.EvalBoolPredicate(expr)
		})
	})
}
