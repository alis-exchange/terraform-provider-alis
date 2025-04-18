# [](https://github.com/alis-exchange/terraform-provider-alis/compare/v2.0.0-beta3...v) (2025-04-18)



# [2.0.0-beta3](https://github.com/alis-exchange/terraform-provider-alis/compare/v2.0.0-beta2...v2.0.0-beta3) (2025-04-18)


### Bug Fixes

* **alis_google_spanner_table:** Refactor `is_stored` change detection logic. ([d609bf4](https://github.com/alis-exchange/terraform-provider-alis/commit/d609bf4d79abbf5bfd9e0d7c48fa1b510fa64925))



# [2.0.0-beta2](https://github.com/alis-exchange/terraform-provider-alis/compare/v2.0.0-beta1...v2.0.0-beta2) (2025-04-18)


### Features

* **spanner:** Add support for `is_stored` attribute in Spanner table resource ([e5c651c](https://github.com/alis-exchange/terraform-provider-alis/commit/e5c651c2e7095c95b373869139888d1d69b0dcc3))



# [2.0.0-beta1](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.5.2...v2.0.0-beta1) (2025-03-27)


### Features

* **spanner:** v2.0.0 ([341e4be](https://github.com/alis-exchange/terraform-provider-alis/commit/341e4bedb8d870c8616c92dc70c32eb929a7a587))



## [1.5.2](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.5.1...v1.5.2) (2025-02-04)



## [1.5.1](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.5.0...v1.5.1) (2025-02-04)



# [1.5.0](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.4.3...v1.5.0) (2024-12-06)


### Features

* **spanner:** add alis_google_spanner_table_foreign_key resource ([fe6b6b3](https://github.com/alis-exchange/terraform-provider-alis/commit/fe6b6b342e3ca60d722b98459652347a6fc14261))



## [1.4.3](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.4.2...v1.4.3) (2024-10-15)


### Bug Fixes

* **spanner:** wrapper proto package names with backticks ([b0ad25a](https://github.com/alis-exchange/terraform-provider-alis/commit/b0ad25abd331b6fbcc60fcae9af3adde6a911f53))



## [1.4.2](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.4.1...v1.4.2) (2024-10-02)


### Features

* **spanner:** add ttl_policy resource ([458be4c](https://github.com/alis-exchange/terraform-provider-alis/commit/458be4c6739c314d782fabc8bb7cf774740a83bb))



## [1.4.1](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.4.0...v1.4.1) (2024-08-29)



# [1.4.0](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.3.4...v1.4.0) (2024-08-29)


### Bug Fixes

* **spanner:** fix spanner table iam binding triggering an apply on permissions reorder ([167243e](https://github.com/alis-exchange/terraform-provider-alis/commit/167243ec11d092a73c38fcc76c8871588faaf89b))



## [1.3.4](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.3.3...v1.3.4) (2024-08-22)


### Bug Fixes

* **spanner:** ensure failure of column_metadata table update doesn't cause an apply failure ([4ead0f3](https://github.com/alis-exchange/terraform-provider-alis/commit/4ead0f3a17ed452d55f2fc0b08b6252495254b31))



## [1.3.3](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.3.2...v1.3.3) (2024-08-15)



## [1.3.2](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.3.1...v1.3.2) (2024-07-29)



## [1.3.1](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.3.0...v1.3.1) (2024-07-26)


### Features

* **spanner:** add support for PROTO enum columns ([5befd97](https://github.com/alis-exchange/terraform-provider-alis/commit/5befd9772244dbdcc410f78b0a691f0e6beff64d))
* **spanner:** add support for PROTO enum columns ([860cd47](https://github.com/alis-exchange/terraform-provider-alis/commit/860cd477c9f81c440c3507d6f4da72551410ce3e))
* **spanner:** add support for PROTO enum columns ([b2d9dfa](https://github.com/alis-exchange/terraform-provider-alis/commit/b2d9dfa9bbe8d895fca649d1a921015e3ef08e95))



# [1.3.0](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.3.0-alpha1...v1.3.0) (2024-07-26)



# [1.3.0-alpha1](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.2.1...v1.3.0-alpha1) (2024-07-24)


### Bug Fixes

* **spanner:** replace table when certain fields are changed ([927c6a3](https://github.com/alis-exchange/terraform-provider-alis/commit/927c6a395f3a9d1a30fc4c592fbed221a43cb7b1))



## [1.2.1](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.2.0...v1.2.1) (2024-07-18)


### Bug Fixes

* **spanner:** replace table when certain fields are changed ([0412d88](https://github.com/alis-exchange/terraform-provider-alis/commit/0412d887c04690f07bdc4162cdfd3547071e99f5))



# [1.2.0](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.1.1...v1.2.0) (2024-07-10)


### Features

* **spanner:** add support for custom database roles and table IAM binding ([67aeb7f](https://github.com/alis-exchange/terraform-provider-alis/commit/67aeb7f6c9ce2a32943d6df545e9c83ea26f7a17))



## [1.1.1](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.1.0...v1.1.1) (2024-07-03)



# [1.1.0](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.0.1...v1.1.0) (2024-06-27)


### Features

* **spanner:** add support for array types ([63c9cbd](https://github.com/alis-exchange/terraform-provider-alis/commit/63c9cbdf44e702c14586ba1a93e57c7847451814))



## [1.0.1](https://github.com/alis-exchange/terraform-provider-alis/compare/v1.0.0...v1.0.1) (2024-06-21)


### Bug Fixes

* fix iam member resources ([4819491](https://github.com/alis-exchange/terraform-provider-alis/commit/48194910b027c30fcc46443579d1b6e68b751b0e))



# [1.0.0](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.9...v1.0.0) (2024-06-14)


### Bug Fixes

* update goreleaser config to v2 ([2236f70](https://github.com/alis-exchange/terraform-provider-alis/commit/2236f703bceac5eaa2b982954431fd3912f785b4))



## [0.0.9](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.8...v0.0.9) (2024-06-14)


### Features

* add support for PROTO datatype to spanner tables ([96f41b8](https://github.com/alis-exchange/terraform-provider-alis/commit/96f41b85d46f329f7af35d30b64303771cfa0903))



## [0.0.8](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.7...v0.0.8) (2024-05-30)



## [0.0.7](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.6...v0.0.7) (2024-05-30)



## [0.0.6](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.5...v0.0.6) (2024-05-24)


### Features

* **discoveryengine:** add google discovery engine schema management ([13e3ba8](https://github.com/alis-exchange/terraform-provider-alis/commit/13e3ba8dacefca3c14fdfa10ff5d577879358b98))



## [0.0.5](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.4-alpha2...v0.0.5) (2024-05-08)



## [0.0.4-alpha1](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.3-alpha5...v0.0.4-alpha1) (2024-05-08)



## [0.0.3-alpha5](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.3-alpha4...v0.0.3-alpha5) (2024-05-08)


### Bug Fixes

* fix error response on tf Read plans. Missing resources should not return an error ([5f02562](https://github.com/alis-exchange/terraform-provider-alis/commit/5f02562ee5a6d721ade48869df8f086f43f4f317))



## [0.0.3-alpha4](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.3-alpha3...v0.0.3-alpha4) (2024-05-08)


### Features

* **spanner:** add spanner table index resource ([d50fe30](https://github.com/alis-exchange/terraform-provider-alis/commit/d50fe3099be210f74ab931e8e99a78b9e9d36eeb))



## [0.0.3-alpha3](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.3-alpha2...v0.0.3-alpha3) (2024-05-06)


### Bug Fixes

* **spanner:** fix UpdateSpannerDatabase refresh computed state ([2d6e044](https://github.com/alis-exchange/terraform-provider-alis/commit/2d6e044ea8df3ed999771d40f5ac9aed5c7b20da))



## [0.0.3-alpha2](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.3-alpha1...v0.0.3-alpha2) (2024-05-06)


### Bug Fixes

* **spanner:** fix UpdateSpannerDatabase update mask validation ([eb48666](https://github.com/alis-exchange/terraform-provider-alis/commit/eb486661a4a1755c50aa7368c45eb78ea8d0751b))



## [0.0.3-alpha1](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.2-alpha9...v0.0.3-alpha1) (2024-05-03)



## [0.0.2-alpha9](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.2-alpha8...v0.0.2-alpha9) (2024-05-03)



## [0.0.2-alpha8](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.2-alpha7...v0.0.2-alpha8) (2024-05-03)



## [0.0.2-alpha7](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.2-alpha6...v0.0.2-alpha7) (2024-05-03)



## [0.0.2-alpha6](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.2-alpha5...v0.0.2-alpha6) (2024-05-03)


### Features

* add support for order key on spanner table index ([fccf6a3](https://github.com/alis-exchange/terraform-provider-alis/commit/fccf6a3f587a7e73304b4e5cbb527486c64478c4))



## [0.0.2-alpha5](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.2-alpha4...v0.0.2-alpha5) (2024-04-30)



## [0.0.2-alpha4](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.2-alpha3...v0.0.2-alpha4) (2024-04-30)



## [0.0.2-alpha3](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.2-alpha2...v0.0.2-alpha3) (2024-04-30)



## [0.0.2-alpha2](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.2-alpha1...v0.0.2-alpha2) (2024-04-29)



## [0.0.2-alpha1](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.1-alpha9...v0.0.2-alpha1) (2024-04-29)


### Bug Fixes

* **bigtable:** fix bigtable gc policy rules ([596d67e](https://github.com/alis-exchange/terraform-provider-alis/commit/596d67e4962ce3ea7a4bc97cd294ddb62b6292d5))



## [0.0.1-alpha9](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.1-alpha8...v0.0.1-alpha9) (2024-04-29)



## [0.0.1-alpha8](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.1-alpha7...v0.0.1-alpha8) (2024-04-29)



## [0.0.1-alpha7](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.1-alpha6...v0.0.1-alpha7) (2024-04-29)



## [0.0.1-alpha6](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.1-alpha5...v0.0.1-alpha6) (2024-04-29)



## [0.0.1-alpha5](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.1-alpha4...v0.0.1-alpha5) (2024-04-29)



## [0.0.1-alpha4](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.1-alpha3...v0.0.1-alpha4) (2024-04-26)



## [0.0.1-alpha3](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.1-alpha2...v0.0.1-alpha3) (2024-04-26)



## [0.0.1-alpha2](https://github.com/alis-exchange/terraform-provider-alis/compare/v0.0.1-alpha1...v0.0.1-alpha2) (2024-04-26)



## [0.0.1-alpha1](https://github.com/alis-exchange/terraform-provider-alis/compare/0f1d86d574ffbb4aa0e8e68b65aadbf4e47b4a81...v0.0.1-alpha1) (2024-04-26)


### Features

* add Release github action ([c608b6b](https://github.com/alis-exchange/terraform-provider-alis/commit/c608b6b78f4d9ef3d89f443e4d711a036a93e05f))
* add Spanner tables, IAM resources ([e54f90b](https://github.com/alis-exchange/terraform-provider-alis/commit/e54f90ba7cf25496b72770f87708e20bc1c1b12a))
* implement spanner database resource ([0f1d86d](https://github.com/alis-exchange/terraform-provider-alis/commit/0f1d86d574ffbb4aa0e8e68b65aadbf4e47b4a81))
* implement spanner table resource ([c6fb12d](https://github.com/alis-exchange/terraform-provider-alis/commit/c6fb12d1f2a813943a921236e4793d187049c7a3))



