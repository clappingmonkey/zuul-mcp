# Changelog

## [0.13.0](https://github.com/clappingmonkey/zuul-mcp/compare/v0.12.2...v0.13.0) (2026-06-15)


### Features

* add get_autohold MCP tool ([#61](https://github.com/clappingmonkey/zuul-mcp/issues/61)) ([441157b](https://github.com/clappingmonkey/zuul-mcp/commit/441157b80fb15e22fd7ed2ec6ceda323c460287c))
* add list_components MCP tool ([#60](https://github.com/clappingmonkey/zuul-mcp/issues/60)) ([5f1afef](https://github.com/clappingmonkey/zuul-mcp/commit/5f1afef944add76a3499918e2749cc8a4f62d76c))


### Documentation

* add Glama score badge ([054ac3c](https://github.com/clappingmonkey/zuul-mcp/commit/054ac3cca645eed0853ddb0176a1a9029f7b8c29))

## [0.12.2](https://github.com/clappingmonkey/zuul-mcp/compare/v0.12.1...v0.12.2) (2026-05-31)


### Bug Fixes

* **deps:** update go dependencies ([#49](https://github.com/clappingmonkey/zuul-mcp/issues/49)) ([b3cd1df](https://github.com/clappingmonkey/zuul-mcp/commit/b3cd1df5dde55b4b6a5e7635cee8bec0adda20b4))


### Documentation

* update dependency workflow after go_deps.from_file() migration ([#51](https://github.com/clappingmonkey/zuul-mcp/issues/51)) ([c54e961](https://github.com/clappingmonkey/zuul-mcp/commit/c54e961b68ba5870b87560335c49db61efe7b90c))

## [0.12.1](https://github.com/clappingmonkey/zuul-mcp/compare/v0.12.0...v0.12.1) (2026-05-31)


### Bug Fixes

* use boolean true for platformCommit instead of string 'enabled' ([0f27ba9](https://github.com/clappingmonkey/zuul-mcp/commit/0f27ba94588468312f245433985f759c1f7e7ab5))

## [0.12.0](https://github.com/clappingmonkey/zuul-mcp/compare/v0.11.0...v0.12.0) (2026-05-30)


### Features

* add custom regex manager to bump Go SDK version in MODULE.bazel ([80214a6](https://github.com/clappingmonkey/zuul-mcp/commit/80214a6b2e5f015dc422648ac9bd51efe6f12757))


### Bug Fixes

* add RENOVATE_REPOSITORIES using github.repository context ([5eb6ae8](https://github.com/clappingmonkey/zuul-mcp/commit/5eb6ae84c5d4d5d6d8e0fa99e3d672cc80423a26))
* pin renovatebot/github-action to v46.1.14 and use full image ([19dbac0](https://github.com/clappingmonkey/zuul-mcp/commit/19dbac00b16bef6a6ee0c790981054198e2a32e3))
* simplify renovate-approve workflow using event context for label check ([683ba4b](https://github.com/clappingmonkey/zuul-mcp/commit/683ba4b77d32a0baaa9e5a36b6d3abd0c0b2c10a))
* use correct renovatebot/github-action version (v46) ([cdac459](https://github.com/clappingmonkey/zuul-mcp/commit/cdac459676e04d8f5f008424ffd2d07978f179a3))
* use RENOVATE_ALLOWED_UNSAFE_EXECUTIONS for Bazel lock file regen ([6a3cd6d](https://github.com/clappingmonkey/zuul-mcp/commit/6a3cd6dd169c409e24580e97d52d01289d8c1824))
* use secrets for app ID and remove renovate-version full ([2577aec](https://github.com/clappingmonkey/zuul-mcp/commit/2577aec9fe6fec344a02d05a7c74f7ecae5c083c))

## [0.11.0](https://github.com/clappingmonkey/zuul-mcp/compare/v0.10.0...v0.11.0) (2026-05-22)


### Features

* add Homebrew tap automation to release workflow ([#33](https://github.com/clappingmonkey/zuul-mcp/issues/33)) ([b1983da](https://github.com/clappingmonkey/zuul-mcp/commit/b1983da6c5118138cee6f588bbe53468e6d2e212))

## [0.10.0](https://github.com/clappingmonkey/zuul-mcp/compare/v0.9.1...v0.10.0) (2026-05-12)


### Features

* add enqueue, dequeue, and promote MCP tools ([#29](https://github.com/clappingmonkey/zuul-mcp/issues/29)) ([921c140](https://github.com/clappingmonkey/zuul-mcp/commit/921c140c350a1c90d73234ade227a131e50fd8a9))

## [0.9.1](https://github.com/clappingmonkey/zuul-mcp/compare/v0.9.0...v0.9.1) (2026-05-12)


### Documentation

* update README with new tools ([#27](https://github.com/clappingmonkey/zuul-mcp/issues/27)) ([9646117](https://github.com/clappingmonkey/zuul-mcp/commit/9646117a7859df31cba12357f8f0f8ed08d4b3c3))

## [0.9.0](https://github.com/clappingmonkey/zuul-mcp/compare/v0.8.0...v0.9.0) (2026-05-12)


### Features

* add get_job_variants MCP tool ([#25](https://github.com/clappingmonkey/zuul-mcp/issues/25)) ([eb24f31](https://github.com/clappingmonkey/zuul-mcp/commit/eb24f318cdeb67459ca69536af1304af6899d6c0))

## [0.8.0](https://github.com/clappingmonkey/zuul-mcp/compare/v0.7.0...v0.8.0) (2026-05-12)


### Features

* add list_semaphores MCP tool ([#22](https://github.com/clappingmonkey/zuul-mcp/issues/22)) ([c8f0603](https://github.com/clappingmonkey/zuul-mcp/commit/c8f0603e58697570be7f41f3d1eb1f7a47e5d9ac))

## [0.7.0](https://github.com/clappingmonkey/zuul-mcp/compare/v0.6.0...v0.7.0) (2026-05-12)


### Features

* add list_connections MCP tool ([#20](https://github.com/clappingmonkey/zuul-mcp/issues/20)) ([8ca7147](https://github.com/clappingmonkey/zuul-mcp/commit/8ca7147983e0fcf06fcc00bf6cf19177991ea4be))

## [0.6.0](https://github.com/clappingmonkey/zuul-mcp/compare/v0.5.0...v0.6.0) (2026-05-12)


### Features

* add list_labels MCP tool ([#19](https://github.com/clappingmonkey/zuul-mcp/issues/19)) ([0b26804](https://github.com/clappingmonkey/zuul-mcp/commit/0b268046be45996d070b29f85347b35c45282b3d))

## [0.5.0](https://github.com/clappingmonkey/zuul-mcp/compare/v0.4.0...v0.5.0) (2026-05-12)


### Features

* add list_nodes MCP tool ([#17](https://github.com/clappingmonkey/zuul-mcp/issues/17)) ([7c5773d](https://github.com/clappingmonkey/zuul-mcp/commit/7c5773db3a2bfc567060c2bb02953ff16eb7429b))

## [0.4.0](https://github.com/clappingmonkey/zuul-mcp/compare/v0.3.0...v0.4.0) (2026-05-12)


### Features

* add get_buildset MCP tool ([#15](https://github.com/clappingmonkey/zuul-mcp/issues/15)) ([3eeda20](https://github.com/clappingmonkey/zuul-mcp/commit/3eeda209c4ebb371e414e9fc4b2fbfd7e9e8848b))

## [0.3.0](https://github.com/clappingmonkey/zuul-mcp/compare/v0.2.2...v0.3.0) (2026-03-18)


### Features

* add --env-file flag for loading configuration from file ([#12](https://github.com/clappingmonkey/zuul-mcp/issues/12)) ([cdd6584](https://github.com/clappingmonkey/zuul-mcp/commit/cdd6584902e555df3c890da9a842da59d1fe7383))

## [0.2.2](https://github.com/clappingmonkey/zuul-mcp/compare/v0.2.1...v0.2.2) (2026-03-17)


### Bug Fixes

* **models:** correct Build and Buildset to match Zuul API response fo… ([#10](https://github.com/clappingmonkey/zuul-mcp/issues/10)) ([232efe0](https://github.com/clappingmonkey/zuul-mcp/commit/232efe0d3e69c103b94305a68f969b22497d150c))

## [0.2.1](https://github.com/clappingmonkey/zuul-mcp/compare/v0.2.0...v0.2.1) (2026-03-17)


### Documentation

* add version badge, CLI options, and stamped build instructions ([#8](https://github.com/clappingmonkey/zuul-mcp/issues/8)) ([8e49657](https://github.com/clappingmonkey/zuul-mcp/commit/8e496574e862d896535a906a4f369729548b5ecd))

## [0.2.0](https://github.com/clappingmonkey/zuul-mcp/compare/v0.1.0...v0.2.0) (2026-03-17)


### Features

* add get_build_logs tool to fetch job output logs ([#3](https://github.com/clappingmonkey/zuul-mcp/issues/3)) ([f174309](https://github.com/clappingmonkey/zuul-mcp/commit/f174309731a1d25f943273bc504a875df65c7de7))


### Bug Fixes

* handle Zuul timestamps without timezone suffix ([#2](https://github.com/clappingmonkey/zuul-mcp/issues/2)) ([823c8d8](https://github.com/clappingmonkey/zuul-mcp/commit/823c8d85317f1514d381c17bd8d5a98e42dd1c2a))
