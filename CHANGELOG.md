# Changelog

## [0.2.0](https://github.com/rshade/controldctl/compare/v0.1.0...v0.2.0) (2026-09-11)


### Added

* add ControlD client construction and ax.Error mapping ([3f99fa2](https://github.com/rshade/controldctl/commit/3f99fa219af365eeae784009c0703c65c252386e))
* add devices list/create/update/delete/types commands ([c5eb212](https://github.com/rshade/controldctl/commit/c5eb2121f7d80f74c6672e2246bcfad8b6601f3f))
* add profile custom rules list/create/update/delete commands ([0ce3417](https://github.com/rshade/controldctl/commit/0ce3417322b0c53a32005f501bdaca9d57d65c6e))
* add profile filters list/update commands ([172de8f](https://github.com/rshade/controldctl/commit/172de8fcd8c82f9721ef3764d64c6505bbb75b69))
* add profile rule folders list/create/update/delete commands ([7e9c037](https://github.com/rshade/controldctl/commit/7e9c0374b5e3a487bea499df2f4d0d770d52295a))
* add profile services list/update commands ([8394cd6](https://github.com/rshade/controldctl/commit/8394cd69344126ffc55310ed7e789cfb22fc8c5f))
* add profiles list/create/update/delete and options commands ([fec49e5](https://github.com/rshade/controldctl/commit/fec49e5dfee07bf3e554bd6adc8f21f55021acc5))
* **cli:** expose deferred ControlD flags and harden rule updates ([7b63b5e](https://github.com/rshade/controldctl/commit/7b63b5e0c5f549fbdaf3b7a7f9ac8cd144bb09f7)), closes [#2](https://github.com/rshade/controldctl/issues/2) [#3](https://github.com/rshade/controldctl/issues/3) [#5](https://github.com/rshade/controldctl/issues/5) [#9](https://github.com/rshade/controldctl/issues/9) [#10](https://github.com/rshade/controldctl/issues/10)
* wire root command, client injection, and --mcp shim ([489461a](https://github.com/rshade/controldctl/commit/489461a7cd5d1357f0d668a6f64762c740678f1d))


### Fixed

* address final review findings across error mapping, schema, tests, docs ([a3872d1](https://github.com/rshade/controldctl/commit/a3872d1501dcaffab56a84ed7594bc2fcea14a74))
* **cli:** normalize the --dry-run payload shape across resources ([d004028](https://github.com/rshade/controldctl/commit/d004028717954f08e1e540f89a85f422fd76848d)), closes [#8](https://github.com/rshade/controldctl/issues/8)
* **deps:** update module github.com/baptistecdr/controld-go to v0.0.11 ([#18](https://github.com/rshade/controldctl/issues/18)) ([5139aee](https://github.com/rshade/controldctl/commit/5139aee4eaaf721c4750be6fbda6e8f54b7ba01f))
* map profile/profile-option payloads through local wrappers ([e7c5dc0](https://github.com/rshade/controldctl/commit/e7c5dc051ccc728d0321c918b4ee2848c489897b))
* remap nested Opt/FilterLevel PK casing in filter payloads ([086fbcd](https://github.com/rshade/controldctl/commit/086fbcd37f1550ce74fd5f350f9f6e9227cef776))


### Changed

* move to standard cmd/controldctl layout ([5aa4cee](https://github.com/rshade/controldctl/commit/5aa4cee15c786fda73ce8bf8d0ca60e737c53ad4))


### Documentation

* add controldctl README with usage examples ([45519f3](https://github.com/rshade/controldctl/commit/45519f3a654009d5cbe1e717588c8eba11cdccaf))
* bootstrap CONTEXT.md and ROADMAP.md ([30c3260](https://github.com/rshade/controldctl/commit/30c3260344395ecfc37d50bd984fdf4e9a3bb7c9))
* drop nonexistent --folder-id flag from profiles rules create example ([fc3ebad](https://github.com/rshade/controldctl/commit/fc3ebad6077ad2cc692a0fb3d882a753d0ba12c3))
* sync roadmap with post-v1 infrastructure work ([8e1f9c0](https://github.com/rshade/controldctl/commit/8e1f9c0b8a099d977cc26b05c29a087445fbacf3))
