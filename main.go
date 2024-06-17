package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"golang.org/x/mod/semver"
)

var (
	token     = os.Getenv("ARGOCD_TOKEN")
	githubPat = os.Getenv("GITHUB_PAT")
	logLevel  = os.Getenv("LOG_LEVEL")
	port      = os.Getenv("PORT")
)

type Request struct {
	Input Input `json:"input"`
}

type Input struct {
	Parameters Parameters `json:"parameters"`
}

type Parameters struct {
	Repository string `json:"repository"`
	// MinRelease is the minimum release that should be returned. If a release is less than this, it will be filtered out
	MinRelease string `json:"min_release"`
	// KeepReleases is the number of releases to keep. If set to 0, all releases will be returned
	KeepReleases int `json:"keep_releases"`
	// OnlyLatestMinor is a flag that if set to true, will only return the latest minor version of a release
	OnlyLatestMinor bool `json:"only_latest_minor"`
	// OnlyLatestPatch is a flag that if set to true, will only return the latest patch version of a release
	// if OnlyLatestMinor is set to true, this flag will be ignored
	OnlyLatestPatch bool `json:"only_latest_patch"`
	// When specified the plugin will return an additional item with the slug `latest`
	// that points to the latest image.
	WithLatest bool `json:"with_latest"`
}

type Output struct {
	Output struct {
		Parameters []Release `json:"parameters"`
	} `json:"output"`
}

func main() {
	if port == "" {
		port = "8080"
	}

	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	l := zerolog.New(os.Stderr).With().Timestamp().Caller().Logger()

	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		l.Warn().Err(err).Msg("failed to parse log level, defaulting to info")
		l = l.Level(zerolog.InfoLevel)
	} else {
		l = l.Level(level)
	}

	if githubPat == "" {
		l.Info().Msg("GITHUB_PAT is not set. This means that private repositories will not be accessible and there is a maximum of 60 requests per hour before being rate limited by Github")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/getparams.execute", generatorHandler(l))

	l.Println(http.ListenAndServe(":"+port, mux))
}

func generatorHandler(l zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := l.WithContext(r.Context())
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		authz := strings.Split(r.Header.Get("Authorization"), " ")
		if len(authz) < 2 || (len(authz) == 2 && authz[1] != token) {
			l.Error().Msgf("%+v != %s", authz, token)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		req := &Request{}
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			l.Error().Err(err).Msg("failed to decode request body")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if !semver.IsValid(req.Input.Parameters.MinRelease) {
			l.Error().Msgf("invalid semver: %s", req.Input.Parameters.MinRelease)
			http.Error(w, "invalid semver. Check https://pkg.go.dev/golang.org/x/mod/semver for details on which are valid versions", http.StatusBadRequest)
		}

		l.Debug().Msgf("fetching releases for %s", req.Input.Parameters.Repository)
		releases, err := getReleases(ctx, req.Input.Parameters.Repository)
		if err != nil {
			l.Error().Err(err).Msg("failed to fetch releases")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		l.Debug().Msgf("fetched %d releases", len(releases))

		filtered, err := getFilteredReleases(releases, req.Input.Parameters)
		if err != nil {
			l.Error().Err(err).Msg("failed to filter releases")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// if withLatest is enabled we add a new release that is a copy of the
		// latest released version but with a `latest` name slug
		if req.Input.Parameters.WithLatest {
			// filtered is an array of Release (not *Release) so this creates a copy
			latest := filtered[len(filtered)-1]
			latest.NameSlug = "latest"
			latest.TagSlug = fmt.Sprintf("%s-latest", latest.TagSlug)
			filtered = append(filtered, latest)
		}

		out := Output{
			Output: struct {
				Parameters []Release `json:"parameters"`
			}{
				Parameters: filtered,
			},
		}

		l.Debug().Msgf("returning %d releases after filtering with min_release of %s", len(out.Output.Parameters), req.Input.Parameters.MinRelease)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(out); err != nil {
			l.Error().Err(err).Msg("failed to encode response")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

type Release struct {
	Name     string `json:"name"`
	NameSlug string `json:"name_slug"`
	TagSlug  string `json:"tag_slug"`
	Commit   Commit `json:"commit"`
	NodeID   string `json:"node_id"`
}

type Commit struct {
	Sha string `json:"sha"`
	URL string `json:"url"`
}

func getFilteredReleases(releases []Release, params Parameters) ([]Release, error) {
	// Sort releases in a descending order so that returning only the latest patch
	// of a minor or latest minor of a major version can be done simply with a map
	sort.SliceStable(releases, func(i, j int) bool {
		return semver.Compare(releases[i].Name, releases[j].Name) > 0
	})

	var (
		filteredReleases []Release
		latestVersion    = map[string]string{}
	)
	for _, r := range releases {
		if semver.Compare(r.Name, params.MinRelease) < 0 {
			continue
		}

		// if we reached the amount of releases we want to keep, break out of the loop
		if params.KeepReleases != 0 && len(filteredReleases) == params.KeepReleases {
			break
		}

		version := semver.MajorMinor(r.Name)
		major := semver.Major(r.Name)

		if params.OnlyLatestMinor {
			if _, ok := latestVersion[major]; !ok {
				latestVersion[major] = r.Name
				r.NameSlug = strings.ReplaceAll(r.Name, ".", "-")
				r.TagSlug = r.NameSlug
				filteredReleases = append(filteredReleases, r)
				continue
			}
			continue
		}

		if params.OnlyLatestPatch {
			if _, ok := latestVersion[version]; !ok {
				latestVersion[version] = r.Name
				r.NameSlug = strings.ReplaceAll(r.Name, ".", "-")
				r.TagSlug = r.NameSlug
				filteredReleases = append(filteredReleases, r)
				continue
			}
			continue
		}

		r.NameSlug = strings.ReplaceAll(r.Name, ".", "-")
		r.TagSlug = r.NameSlug
		filteredReleases = append(filteredReleases, r)
	}

	// sort the releases by increasing order before returning them
	sort.SliceStable(filteredReleases, func(i, j int) bool {
		return semver.Compare(filteredReleases[i].Name, filteredReleases[j].Name) < 0
	})

	return filteredReleases, nil
}

func getReleases(ctx context.Context, repo string) ([]Release, error) {
	l := log.Ctx(ctx)

	rr, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+repo+"/tags", nil)
	if err != nil {
		return nil, err
	}

	if githubPat != "" {
		rr.Header.Set("Authorization", "Bearer "+githubPat)
	}

	res, err := http.DefaultClient.Do(rr)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		l.Error().Int("github_status_code", res.StatusCode).Msg("failed to fetch releases")
		return nil, fmt.Errorf("failed to fetch releases, github responded with: %d", res.StatusCode)
	}

	var releases []Release
	if err := json.NewDecoder(res.Body).Decode(&releases); err != nil {
		return nil, err
	}

	return releases, nil
}
