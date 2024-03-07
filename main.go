package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	MinRelease string `json:"min_release"`
}

type Output struct {
	Parameters []Release `json:"parameters"`
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

	mux.HandleFunc("/api/v1/getparams.execute", func(w http.ResponseWriter, r *http.Request) {
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

		out := Output{
			Parameters: getFilteredReleases(releases, req.Input.Parameters.MinRelease),
		}

		l.Debug().Msgf("returning %d releases after filtering with min_release of %s", len(out.Parameters), req.Input.Parameters.MinRelease)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(out); err != nil {
			l.Error().Err(err).Msg("failed to encode response")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	l.Println(http.ListenAndServe(":"+port, mux))
}

type Release struct {
	Name   string `json:"name"`
	Commit Commit `json:"commit"`
	NodeID string `json:"node_id"`
}

type Commit struct {
	Sha string `json:"sha"`
	URL string `json:"url"`
}

func getFilteredReleases(releases []Release, minRelease string) []Release {
	var filteredReleases []Release
	for _, r := range releases {
		if semver.Compare(r.Name, minRelease) > 0 {
			filteredReleases = append(filteredReleases, r)
		}
	}

	return filteredReleases
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
