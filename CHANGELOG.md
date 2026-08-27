# Changelog

## [1.1.1](https://github.com/episki/episki-cli/compare/v1.1.0...v1.1.1) (2026-08-27)


### Bug Fixes

* **deps:** Bump the actions group with 4 updates ([#4](https://github.com/episki/episki-cli/issues/4)) ([bfc3b3c](https://github.com/episki/episki-cli/commit/bfc3b3c8edc26f2099f57a196daf3c60ba553607))

## [1.1.0](https://github.com/episki/episki-cli/compare/v1.0.0...v1.1.0) (2026-08-27)


### Features

* **evidence:** upload a file as evidence from the terminal ([9fbf049](https://github.com/episki/episki-cli/commit/9fbf0492c38e2c76b68875b8301ca8ba103b5d0f))
* **resources:** vendors, programs, obligations, exceptions, and goals ([eb22aa6](https://github.com/episki/episki-cli/commit/eb22aa69d28e8e61ff0cb084a603f233d656f04d))


### Bug Fixes

* **auth:** sign in with the emailed code, not the emailed link ([37d0bc9](https://github.com/episki/episki-cli/commit/37d0bc91922b3cc5c75ff0f61858b714a5f94773))
* **resources:** restore --archived, and stop leaking PGRST116 at users ([089f47c](https://github.com/episki/episki-cli/commit/089f47c9cc4c60c4e5728eddb24a94254c601a10))


### Miscellaneous Chores

* release 1.1.0 ([c594e03](https://github.com/episki/episki-cli/commit/c594e03cf6e4c6f393d68fae75f8a1b02df88d84))

## 1.0.0 (2026-07-05)


### Features

* **auth:** auth refresh command and workspace-claim decoding ([83d30cc](https://github.com/episki/episki-cli/commit/83d30ccb00e70cafefb78895c43dce6b6a4f3d16))
* **auth:** branded sign-in pages for the loopback flow ([fea6e45](https://github.com/episki/episki-cli/commit/fea6e454b3956b58a581a7db996325c78c940e1c))
* **auth:** magic-link email login; loopback redirect without path ([263ecaf](https://github.com/episki/episki-cli/commit/263ecaff3ff2f5f602724e424ce298fbd5a320af))
* **config:** default to the production episki project ([1e92161](https://github.com/episki/episki-cli/commit/1e921619e0ce6cb85a97a436a0df95e89eb11680))
* **resources:** workspace, GRC entity, and work-item commands ([4e97a08](https://github.com/episki/episki-cli/commit/4e97a089ca758cfffe935bcce8ab02c9ff07aff1))


### Bug Fixes

* **auth:** surface GoTrue error details; 15-minute timeout for email login ([b8215ff](https://github.com/episki/episki-cli/commit/b8215ff51021170153ae0beac89b641db4b427b9))
