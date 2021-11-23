package command

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/go-test/deep"
	"github.com/hashicorp/vault/api"
	"github.com/mitchellh/cli"
)

func testKVMetadataPatchCommand(tb testing.TB) (*cli.MockUi, *KVMetadataPatchCommand) {
	tb.Helper()

	ui := cli.NewMockUi()
	return ui, &KVMetadataPatchCommand{
		BaseCommand: &BaseCommand{
			UI: ui,
		},
	}
}

func kvMetadataPatchWithRetry(t *testing.T, client *api.Client, args []string, stdin *io.PipeReader) (int, string) {
	t.Helper()

	return retryKVCommand(t, func() (int, string) {
		ui, cmd := testKVMetadataPatchCommand(t)
		cmd.client = client

		if stdin != nil {
			cmd.testStdin = stdin
		}

		code := cmd.Run(args)
		combined := ui.OutputWriter.String() + ui.ErrorWriter.String()

		return code, combined
	})
}

func kvMetadataPutWithRetry(t *testing.T, client *api.Client, args []string, stdin *io.PipeReader) (int, string) {
	t.Helper()

	return retryKVCommand(t, func() (int, string) {
		ui, cmd := testKVMetadataPutCommand(t)
		cmd.client = client

		if stdin != nil {
			cmd.testStdin = stdin
		}

		code := cmd.Run(args)
		combined := ui.OutputWriter.String() + ui.ErrorWriter.String()

		return code, combined
	})
}

func TestKvMetadataPatchEmptyArgs(t *testing.T) {
	client, closer := testVaultServer(t)
	defer closer()

	if err := client.Sys().Mount("kv/", &api.MountInput{
		Type: "kv-v2",
	}); err != nil {
		t.Fatalf("kv-v2 mount error: %#v", err)
	}

	args := make([]string, 0)
	code, combined := kvMetadataPatchWithRetry(t, client, args, nil)

	expectedCode := 1
	expectedOutput := "Not enough arguments"

	if code != expectedCode {
		t.Fatalf("expected code to be %d but was %d for patch cmd with args %#v", expectedCode, code, args)
	}

	if !strings.Contains(combined, expectedOutput) {
		t.Fatalf("expected output to be %q but was %q for patch cmd with args %#v", expectedOutput, combined, args)
	}
}

func TestKvMetadataPatchFlags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		args     []string
		out      string
		code     int
		expected map[string]interface{}
	}{
		{
			"cas_required",
			[]string{"-cas-required=true"},
			"Success!",
			0,
			map[string]interface{}{
				"cas_required": true,
			},
		},
		{
			"custom_metadata",
			[]string{"-custom-metadata=baz=quux"},
			"Success!",
			0,
			map[string]interface{}{
				"custom_metadata": map[string]interface{}{
					"baz": "quux",
				},
			},
		},
		{
			"delete_version_after",
			[]string{"-delete-version-after=5s"},
			"Success!",
			0,
			map[string]interface{}{
				"delete_version_after": "5s",
			},
		},
		{
			"max_versions",
			[]string{"-max-versions=10"},
			"Success!",
			0,
			map[string]interface{}{
				"max_versions": json.Number("10"),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, closer := testVaultServer(t)
			defer closer()

			basePath := t.Name() + "/"
			secretPath := basePath + "my-secret"
			metadataPath := basePath + "metadata/" + "my-secret"

			if err := client.Sys().Mount(basePath, &api.MountInput{
				Type: "kv-v2",
			}); err != nil {
				t.Fatalf("kv-v2 mount error: %#v", err)
			}

			code, combined := kvMetadataPutWithRetry(t, client, []string{secretPath}, nil)

			if code != 0 {
				t.Fatalf("initial metadata put failed, code: %d, output: %s", code, combined)
			}

			initialMetadata, err := client.Logical().Read(metadataPath)
			if err != nil {
				t.Fatalf("metadata read failed, err: %#v", err)
			}

			args := append(tc.args, secretPath)

			code, combined = kvMetadataPatchWithRetry(t, client, args, nil)

			if !strings.Contains(combined, tc.out) {
				t.Fatalf("expected output to be %q but was %q for patch cmd with args %#v", tc.out, combined, args)
			}
			if code != tc.code {
				t.Fatalf("expected code to be %d but was %d for patch cmd with args %#v", tc.code, code, args)
			}

			patchedMetadata, err := client.Logical().Read(metadataPath)
			if err != nil {
				t.Fatalf("metadata read failed, err: %#v", err)
			}

			for k, v := range patchedMetadata.Data {
				if k == "versions" {
					continue
				}

				var expectedVal interface{}

				if inputVal, ok := tc.expected[k]; ok {
					expectedVal = inputVal
				} else {
					expectedVal = initialMetadata.Data[k]
				}

				if k == "custom_metadata" {
					if diff := deep.Equal(expectedVal, v); len(diff) > 0 {
						t.Fatalf("patched \"custom_metadata\" mismatch, diff: %#v", diff)
					}
				} else if expectedVal != v {
					t.Fatalf("patched key %q mismatch, expected: %#v, actual %#v", k, expectedVal, v)
				}
			}
		})
	}
}
