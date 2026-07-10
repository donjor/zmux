# Trusted runtime-config fixture

This disposable project exercises canonical `pi-zmux` trusted project configuration.
Launch the N-013 worker from this directory and ensure Pi marks the project trusted.
If it is untrusted, the extension intentionally ignores `.pi/zmux.json` and reports
`project-untrusted`; that is an invalid harness setup, not a dispatcher regression.
