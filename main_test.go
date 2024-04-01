package main

import (
	"reflect"
	"testing"
)

func TestGetFilteredReleases(t *testing.T) {
	cases := []struct {
		name             string
		params           Parameters
		releases         []Release
		expectedReleases []Release
	}{
		{
			name: "no filter",
			params: Parameters{
				MinRelease: "v0.0.0",
			},
			releases: []Release{
				{Name: "v0.0.0"},
				{Name: "v0.0.1"},
				{Name: "v0.1.0"},
				{Name: "v1.0.0"},
			},
			expectedReleases: []Release{
				{Name: "v0.0.0", NameSlug: "v0-0-0"},
				{Name: "v0.0.1", NameSlug: "v0-0-1"},
				{Name: "v0.1.0", NameSlug: "v0-1-0"},
				{Name: "v1.0.0", NameSlug: "v1-0-0"},
			},
		},
		{
			name: "only latest minor",
			params: Parameters{
				MinRelease:      "v0.0.0",
				OnlyLatestMinor: true,
			},
			releases: []Release{
				{Name: "v0.0.0"},
				{Name: "v0.0.1"},
				{Name: "v0.1.0"},
				{Name: "v0.1.1"},
				{Name: "v1.0.0"},
				{Name: "v1.0.1"},
			},
			expectedReleases: []Release{
				{Name: "v0.1.1", NameSlug: "v0-1-1"},
				{Name: "v1.0.1", NameSlug: "v1-0-1"},
			},
		},
		{
			name: "only latest patch",
			params: Parameters{
				MinRelease:      "v0.0.0",
				OnlyLatestPatch: true,
			},
			releases: []Release{
				{Name: "v0.0.0"},
				{Name: "v0.0.1"},
				{Name: "v0.1.0"},
				{Name: "v0.1.1"},
			},
			expectedReleases: []Release{
				{Name: "v0.0.1", NameSlug: "v0-0-1"},
				{Name: "v0.1.1", NameSlug: "v0-1-1"},
			},
		},
		{
			name: "only latest minor has precedence",
			params: Parameters{
				MinRelease:      "v0.0.0",
				OnlyLatestMinor: true,
				OnlyLatestPatch: true,
			},
			releases: []Release{
				{Name: "v0.0.0"},
				{Name: "v0.0.1"},
				{Name: "v0.1.0"},
				{Name: "v0.1.1"},
				{Name: "v1.0.0"},
				{Name: "v1.0.1"},
			},
			expectedReleases: []Release{
				{Name: "v0.1.1", NameSlug: "v0-1-1"},
				{Name: "v1.0.1", NameSlug: "v1-0-1"},
			},
		},
		{
			name: "only latest minor combined with min release",
			params: Parameters{
				MinRelease:      "v0.1.2",
				OnlyLatestMinor: true,
			},
			releases: []Release{
				{Name: "v0.0.0"},
				{Name: "v0.0.1"},
				{Name: "v0.1.0"},
				{Name: "v0.1.1"},
				{Name: "v1.0.0"},
				{Name: "v1.0.1"},
			},
			expectedReleases: []Release{
				{Name: "v1.0.1", NameSlug: "v1-0-1"},
			},
		},
		{
			name: "keep releases returns only the amount of releases we want to keep",
			params: Parameters{
				MinRelease:   "v0.1.2",
				KeepReleases: 2,
			},
			releases: []Release{
				{Name: "v0.0.0"},
				{Name: "v0.0.1"},
				{Name: "v0.1.0"},
				{Name: "v0.1.1"},
				{Name: "v1.0.0"},
				{Name: "v1.0.1"},
			},
			expectedReleases: []Release{
				{Name: "v1.0.0", NameSlug: "v1-0-0"},
				{Name: "v1.0.1", NameSlug: "v1-0-1"},
			},
		},
		{
			name: "keep releases plus only latest minor returns right releases",
			params: Parameters{
				MinRelease:      "v0.1.0",
				KeepReleases:    1,
				OnlyLatestMinor: true,
			},
			releases: []Release{
				{Name: "v0.0.0"},
				{Name: "v0.0.1"},
				{Name: "v0.1.0"},
				{Name: "v0.1.1"},
				{Name: "v1.0.0"},
				{Name: "v1.0.1"},
			},
			expectedReleases: []Release{
				{Name: "v1.0.1", NameSlug: "v1-0-1"},
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			releases, err := getFilteredReleases(test.releases, test.params)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(releases, test.expectedReleases) {
				t.Errorf("expected releases to be %v, got %v", test.expectedReleases, releases)
			}
		})
	}
}
