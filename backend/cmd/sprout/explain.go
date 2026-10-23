package main

// Plain-language explanations for the stable failure codes a deployment can
// end with. Kept in step by hand with failureExplanations in
// frontend/src/components/deployments/deployment-detail.tsx, worded for a
// terminal instead of a drop zone.
var failureExplanations = map[string]string{
	"node_contract_invalid":               "The folder doesn't follow the Node.js app rules: check package.json, the lockfile, and the build and start scripts.",
	"node_build_dependencies_unsupported": "This app uses npm packages, and Sprout can't install packages yet. Plain websites, and Node.js apps that use no packages, work today.",
	"node_build_failed":                   "The app's build script failed.",
	"node_build_artifact_invalid":         "The build didn't produce a usable dist/ folder.",
	"node_build_artifact_too_large":       "The build output is larger than the limit.",
	"node_build_docker_unavailable":       "Docker or the pinned Node image isn't available on the Sprout server.",
	"source_archive_invalid":              "The upload wasn't a readable archive. Deploy again.",
	"source_limit_exceeded":               "The folder has too many files or is too large (up to 256 files and 8 MiB).",
	"source_sensitive_path":               "The folder contains a sensitive file such as a key or .env.",
	"source_unavailable":                  "The upload was no longer available. Deploy again.",
	"process_interrupted":                 "The server restarted while this was running.",
	"job_timeout":                         "It took longer than the time limit.",
	"node_contract_engine":                `package.json needs "engines": { "node": "24.x" } so Sprout knows which Node.js version to use.`,
	"node_contract_private":               `package.json needs "private": true.`,
	"node_contract_scripts":               `package.json needs both a "build" script and a "start" script.`,
	"node_contract_manifest":              "package.json is missing, unreadable, or has no name and version.",
	"node_contract_lockfile":              `package-lock.json is missing or doesn't match package.json. Run "npm install" and include the file.`,
	"node_contract_workspaces":            "Workspaces (monorepos) aren't supported yet.",
	"node_contract_files":                 "The folder contains files Sprout doesn't accept in a Node.js app (such as a Dockerfile).",
	"static_contract_invalid":             "It is neither a Node.js app nor a valid static site. A static site needs index.html at the root, no package.json, and plain file names (letters, numbers, dots, dashes, underscores).",
	"provision_failed":                    "The built app could not be stored for running.",
	"release_failed":                      "The app could not be made ready to serve.",
	"health_check_failed":                 "The app started but did not answer GET /health with 200 within 30 seconds. Make sure it listens on 0.0.0.0 and the PORT environment variable. Any earlier version is still serving.",
	"runtime_unavailable":                 "Sprout could not reach Docker to run the app. Is Docker running on the Sprout server?",
}

// explain describes a failure code, keeping the code itself so it can be
// searched for and reported.
func explain(code string) string {
	if text, ok := failureExplanations[code]; ok {
		return text + " (" + code + ")"
	}
	return "The deployment failed (" + code + ")."
}
