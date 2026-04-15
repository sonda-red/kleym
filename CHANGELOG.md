## [1.3.0](https://github.com/sonda-red/kleym/compare/v1.2.1...v1.3.0) (2026-04-15)

### Features

* add unit tests for per-objective collision reconciliation logic ([75b4b12](https://github.com/sonda-red/kleym/commit/75b4b12203cfa2f6b2ff3adf0d8fde368082a9a4))
* implement per-objective identity collision detection in reconciliation ([d67b7cf](https://github.com/sonda-red/kleym/commit/d67b7cfb26805fd10cb078831e613e5a289c0c9e))

### Documentation

* clarify scope boundary and add supported downstream patterns in spec.md ([930648d](https://github.com/sonda-red/kleym/commit/930648d727144b24aa393c5ec46c519fdf378896))
* enhance documentation with a comprehensive repository map and contributing guidelines ([8477b41](https://github.com/sonda-red/kleym/commit/8477b41a01044203cefcf5a9ce100d6ade5f7b7a))

### Code Refactoring

* simplify return values in inference reconciliation methods ([9acb270](https://github.com/sonda-red/kleym/commit/9acb27094794959c09d9e7c30d19870d2c6c2f1a))
* simplify return values in resolveInferenceObjective and resolveInferencePool methods ([30e588c](https://github.com/sonda-red/kleym/commit/30e588c50917c158c50aa8f1e1b148b4d2e4ab5c))
* standardize test setup by removing namespace parameters and using default values ([29375d0](https://github.com/sonda-red/kleym/commit/29375d0fa1ca652a0a80be4159fc7efe4bb49b5a))

## [1.2.1](https://github.com/sonda-red/kleym/compare/v1.2.0...v1.2.1) (2026-04-14)

### Code Refactoring

* consolidate CI workflows into a single configuration file ([6d16e47](https://github.com/sonda-red/kleym/commit/6d16e47653355139d641a1df34e8ab9dc3774a2c))
* consolidate lint and test jobs into a single verify job ([a733968](https://github.com/sonda-red/kleym/commit/a73396804ea37354172b0095d8f2a6985a42f207))

## [1.2.0](https://github.com/sonda-red/kleym/compare/v1.1.3...v1.2.0) (2026-04-14)

### Features

* implement reconciliation logic for InferenceIdentityBinding with finalizer and event recording ([a33eba1](https://github.com/sonda-red/kleym/commit/a33eba16121775fc17a5177b6c8efcc9becb0c50))

### Bug Fixes

* add k8s.io/api dependency to go.mod ([3e035e5](https://github.com/sonda-red/kleym/commit/3e035e53d45c514676a881667b25aa750988bf71))
* remove scheduled cron job from auto update workflow ([c862876](https://github.com/sonda-red/kleym/commit/c86287635dd5bddcb855c5208c43971d5d9124f8))
* update ClusterRole to include additional rules for inference and spire resources ([8479912](https://github.com/sonda-red/kleym/commit/84799120a7914f447430eb065f644d0ba017772b))
* update golangci-lint configuration and version, improve caching setup ([1a48b3c](https://github.com/sonda-red/kleym/commit/1a48b3c50786382b7917cffd16cdc6059338de01))

### Code Refactoring

* replace hardcoded values with constants for improved maintainability ([820e4ee](https://github.com/sonda-red/kleym/commit/820e4ee19c408f9079c98e9fc238ca3717649680))

## [1.1.3](https://github.com/sonda-red/kleym/compare/v1.1.2...v1.1.3) (2026-04-08)

### Bug Fixes

* add validation rules for InferenceIdentityBinding CRD fields ([cd42b0f](https://github.com/sonda-red/kleym/commit/cd42b0fb9b347ef6b861767c5a59e17932bff9d5))
* update .gitignore to include .codex ([ae52282](https://github.com/sonda-red/kleym/commit/ae5228298fa3776dd3887c05cf856c06a4406c03))

## [1.1.2](https://github.com/sonda-red/kleym/compare/v1.1.1...v1.1.2) (2026-04-06)

### Bug Fixes

* **deps:** update all dependencies ([4bd4fe3](https://github.com/sonda-red/kleym/commit/4bd4fe3f607926fb468f4454c1f8fa76f2d19d85))

## [1.1.1](https://github.com/sonda-red/kleym/compare/v1.1.0...v1.1.1) (2026-03-08)

### Bug Fixes

* **deps:** update all dependencies ([#46](https://github.com/sonda-red/kleym/issues/46)) ([56c8345](https://github.com/sonda-red/kleym/commit/56c83453fbd21e334a6c96301a503b94a2f5a3c9))

## [1.1.0](https://github.com/sonda-red/kleym/compare/v1.0.3...v1.1.0) (2026-02-22)

### Features

* enhance InferenceIdentityBinding API with container discrimination and SPIFFE ID generation ([7dbcb24](https://github.com/sonda-red/kleym/commit/7dbcb24f5ccac1bb56e64fd8816d785109753711))
* expand InferenceIdentityBinding API with container discrimination, identity modes, and selector templates ([f127b43](https://github.com/sonda-red/kleym/commit/f127b438ccb12c05843871838d23dd966d4697b8))
* implement DeepCopy methods for ComputedSpiffeIDStatus, ContainerDiscriminator, and RenderedSelectorStatus; update InferenceIdentityBinding YAML sample ([6b77cfc](https://github.com/sonda-red/kleym/commit/6b77cfc44373a70e237519c17b3027f2e922808c))
* update image URL in Makefile and add deploy-dev target for development deployment ([519caea](https://github.com/sonda-red/kleym/commit/519caea53b5c6f172b47eea485dcf8276c052391))
* update InferenceIdentityBinding controller tests with spec field population ([6816f15](https://github.com/sonda-red/kleym/commit/6816f159a6f8183773c22efda7fb3e02545e7a42))

### Documentation

* enhance identity model section with detailed identity boundaries and container level enforcement ([65649f2](https://github.com/sonda-red/kleym/commit/65649f289e6c859284707bfbf8f6d158ea38a9ae))

## [1.0.3](https://github.com/sonda-red/kleym/compare/v1.0.2...v1.0.3) (2026-02-21)

### Bug Fixes

* update GOFLAGS and GOMAXPROCS for improved linting performance ([06c3148](https://github.com/sonda-red/kleym/commit/06c3148e61843fd53a85e328a88dc98b3c6c4626))

## [1.0.2](https://github.com/sonda-red/kleym/compare/v1.0.1...v1.0.2) (2026-02-21)

### Bug Fixes

* revert Go version in lint workflow to 1.25 ([619f0de](https://github.com/sonda-red/kleym/commit/619f0de8dddaa00a6a5cc9a0f390c06aadae54ea))

## [1.0.1](https://github.com/sonda-red/kleym/compare/v1.0.0...v1.0.1) (2026-02-19)

### Bug Fixes

* **deps:** update go dependencies ([#41](https://github.com/sonda-red/kleym/issues/41)) ([214be6c](https://github.com/sonda-red/kleym/commit/214be6c5d2d9e28984b9a5ef0f514d4a37e980a8))

## 1.0.0 (2026-02-18)

### Features

* add auto-update workflow for Kubebuilder project ([0defcbd](https://github.com/sonda-red/kleym/commit/0defcbdfcb818680ba22d7e99b266496888d32c8))
* add InferenceTrustBinding API and controller implementation ([18ab68b](https://github.com/sonda-red/kleym/commit/18ab68b0bc18386fadb88deb33850be4bce582cf))
* add InferenceTrustBinding CRD and update manager role permissions ([aa297a6](https://github.com/sonda-red/kleym/commit/aa297a690df0b86e3bb1aff03d5355c19d37a1a3))

### Documentation

* enhance README with detailed project scope, core values, and MVP design targets ([9470a4e](https://github.com/sonda-red/kleym/commit/9470a4e22a46f9e2645ff9c16d364c95bccfe428))
* refine purpose and core problem sections in spec document for clarity ([0853d3d](https://github.com/sonda-red/kleym/commit/0853d3d71bb20b036febac93dbca7f76fd0f6ec7))
* update README and add spec document for project purpose and details ([d3b8d49](https://github.com/sonda-red/kleym/commit/d3b8d49c0bc86a69dd9ced725f21c57d105e2330))
