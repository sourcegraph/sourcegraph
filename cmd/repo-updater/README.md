# repo-updater

Repo-updater tracks the state of repos, and is responsible for automatically scheduling updates ("git fetch" runs) using gitserver. Other apps which desire updates or fetches should be telling repo-updater, rather than using gitserver directly, so repo-updater can take their changes into account. It is a singleton service by design, so there must only be one replica.
Hello World
