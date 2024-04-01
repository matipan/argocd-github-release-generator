# argocd-github-release-generator
`argocd-github-release-generator` is an [ArgoCD Plugin Generator](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/Generators-Plugin/) for `ApplicationSet`s that generates an ArgoCD application for each Github Release on a given repository.

## Installing
> [!NOTE]
> For installing this project we assume that you have a k8s cluster with ArgoCD installed up and running.

This project is composed of 4 components:
- [Secret](k8s/install.yaml#L1): stores the ArgoCD Token (used to authorize requests coming from ArgoCD) and an optional GITHUB_PAT (used to access private repositories and/or have a bigger rate limit).
- [ConfigMap](k8s/install.yaml#L13): contains the plugin configuration that ArgoCD reads to know how to communicate and authenticate with it.
- [Deployment](k8s/install.yaml#L22): deploys the actual plugin that responds to HTTP requests coming from ArgoCD.
- [Service](k8s/install.yaml#L69): exposes the deployment internally for ArgoCD to reach it.

To install it in your cluster you can run:
```terminal
$ ARGOCD_TOKEN="$(echo -n '<strong_password>' | base64)" envsubst < https://raw.githubusercontent.com/matipan/argocd-github-release-generator/v0.0.3/k8s/install.yaml | k apply -f -
```

> [!TIP]
> If you plan to watch private repositories or have a refresh interval lower than 1 per minute then you must specify a GITHUB_PAT.
> `$ GITHUB_PAT=<YOUR_PAT> ARGOCD_TOKEN="$(echo -n '<strong_password>' | base64)" envsubst < https://raw.githubusercontent.com/matipan/argocd-github-release-generator/v0.0.3/k8s/install.yaml | k apply -f -`

## Setting up your ApplicationSet

The plugin receives two parameters that you must configure:
- `repository`: specify the repository that should be used to look for releases.
- `min_release`: specify the starting point for the releases. This value is useful to control how many applications you generate and remove applications related to old releases.

You can optionally configure the following three parameters to further control which releases should be returned:
- `keep_releases`: specify how many releases should be kept. If you set this value to `3` then the plugin will only return the 3 most recent releases.
- `only_latest_minor`: if set to `true` then the plugin will only return the latest release for each major version.
- `only_latest_patch`: if set to `true` then the plugin will only return the latest release for each minor version. This parameter is ignored if `only_latest_minor` is set to `true`.

> [!NOTE]
> At the moment this project only supports releases that follow `semver` (e.g `v0.1.0`)

With plugin generators you also configure the interval with which ArgoCD polls this plugin. Keep this interval in mind given that each request to the plugin will make a request to Github's API. If you have not configured a GITHUB_PAT then you have a rate limit of 60 requests per hour (one per minute).

An example ApplicationSet that watches the [dagger/dagger](https://github.com/dagger/dagger) repository every minute and creates applications for releases starting at `v0.9.8`:
```
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: <YOUR_APPLICATION_SET_NAME>
spec:
  generators:
    - plugin:
        configMapRef:
          # Specify argocd where the configuration of this plugin can be found
          name: argocd-github-release-generator
        input:
          parameters:
            repository: "dagger/dagger"
            min_release: v0.9.8
        requeueAfterSeconds: 60

  template:
    [Your application configuration]
```
