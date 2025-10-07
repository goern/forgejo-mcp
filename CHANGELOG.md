## [2.2.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.1.0...v2.2.0) (2025-10-07)

### :sparkles: Features

* deprecate GITEA_* environment variables in favor of FORGEJO_* ([d7c54ac](https://codeberg.org/goern/forgejo-mcp/commit/d7c54acae957ae0408e9e951e93e3308a1ba6630))
* implement comprehensive MCP server logging improvements ([97fce8f](https://codeberg.org/goern/forgejo-mcp/commit/97fce8fa0beb4a713d291d810adcb500734e58e2))

### :memo: Documentation

* update README.md to use forgejo.example.org instead of forgejo.org ([d760b2c](https://codeberg.org/goern/forgejo-mcp/commit/d760b2c4c22fb3db48168d511d2558e2ecb10120))

### :zap: Refactor

* replace host/port flags with url and separate sse-port ([2d59d29](https://codeberg.org/goern/forgejo-mcp/commit/2d59d2957213de57e4dffec5dd95babf1c0d3d82))

### :repeat: Chore

* **deps:** update golang docker tag to v1.25 ([b0a4c71](https://codeberg.org/goern/forgejo-mcp/commit/b0a4c71a9dc13a8ac103534b43bdf2898df65a2f))
* **deps:** update golang:1.24-alpine docker digest to c8c5f95 ([4d335c6](https://codeberg.org/goern/forgejo-mcp/commit/4d335c6c271b298c171bb55e0b145f64e2ba28b1))
* **deps:** update golang:1.24-alpine docker digest to daae04e ([df47a38](https://codeberg.org/goern/forgejo-mcp/commit/df47a38b78319d8d34d9643bf602c52c88ea812a))
* **deps:** update golang:1.24-alpine docker digest to ddf5200 ([fc69200](https://codeberg.org/goern/forgejo-mcp/commit/fc692001d042e9add0e3b5f73bf015e10f20684c))
* **deps:** update golang:1.25-alpine docker digest to 2ad042d ([bcb65c4](https://codeberg.org/goern/forgejo-mcp/commit/bcb65c4907a0aed33f3ac72b4b309f30fb06c020))
* **deps:** update golang:1.25-alpine docker digest to b6ed3fd ([cc440cf](https://codeberg.org/goern/forgejo-mcp/commit/cc440cf3135de1bdf333bb845a27f21feeb8bd81))
* **deps:** update golang:1.25-alpine docker digest to f18a072 ([ce5dd65](https://codeberg.org/goern/forgejo-mcp/commit/ce5dd6530f285dcdc8c3444cb37b09dcc598f629))
* remove air part from Makefile ([2bba853](https://codeberg.org/goern/forgejo-mcp/commit/2bba8531a962c06b87c921e58ee43d0e727fd397))

## [2.1.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.0.0...v2.1.0) (2025-07-01)

### :sparkles: Features

* add owner/organization support for repository creation ([8acc73f](https://codeberg.org/goern/forgejo-mcp/commit/8acc73fdcfbb9d1f265acfb69d83089110e25e06)), closes [#17](https://codeberg.org/goern/forgejo-mcp/issues/17)

### :repeat: Chore

* **deps:** update golang:1.24-alpine docker digest to 68932fa ([29d9359](https://codeberg.org/goern/forgejo-mcp/commit/29d93596e7c0d3a04ad00c2d4aa9dc2f69b85acc))
* **deps:** update golang:1.24-alpine docker digest to b4f875e ([2927727](https://codeberg.org/goern/forgejo-mcp/commit/2927727951b1879fc113db34aed7c72f3470b89e))
* **deps:** update golang:1.24-alpine docker digest to ef18ee7 ([97bff39](https://codeberg.org/goern/forgejo-mcp/commit/97bff393bd2aeabc5f6ab4f18159926488c42bf0))

## 2.0.0 (2025-04-24)                                                                                                                                                                                                           
                                                                                                                                                                                                                                       
### ✨  Features                                                                                                                                                                                                                       
                                                                                                                                                                                                                                       
    * rebase on https://codeberg.org/fraschm98/forgejo-mcp (9e8edcd (https://codeberg.org/goern/forgejo-mcp/commit/9e8edcd5514c5808798239c09579f390d350082f))

## [1.2.0](https://codeberg.org/goern/forgejo-mcp/compare/v1.1.0...v1.2.0) (2025-04-09)

### :sparkles: Features

- add smithery.ai integration ([4a46279](https://codeberg.org/goern/forgejo-mcp/commit/4a462797690f0c1b81f1ed83bed1853b7dfb1861))

### :bug: Fixes

- release pipeline sequence ([7ebc987](https://codeberg.org/goern/forgejo-mcp/commit/7ebc987c741cad5271eeb1be34ef82bcded2654d))

## [1.1.0](https://codeberg.org/goern/forgejo-mcp/compare/v1.0.0...v1.1.0) (2025-04-09)

### :sparkles: Features

- add a project logo ([8dac350](https://codeberg.org/goern/forgejo-mcp/commit/8dac3505d31046f23eb4de9744d888c307e9432b))
- **api:** add detailed schema for update_issue endpoint 🎯🛠️✨ ([9199474](https://codeberg.org/goern/forgejo-mcp/commit/919947445ce7dd82264d2405d55dd5ee84208b07))

### :bug: Fixes

- the changelog ([483f544](https://codeberg.org/goern/forgejo-mcp/commit/483f5441a585ecced82ff769fc647a96fb4fe136))

### :repeat: Chore

- just small refactorings ([5437bcc](https://codeberg.org/goern/forgejo-mcp/commit/5437bcce9c15741fea5df54d0df3b46a0e17b063))
- **release:** 1.1.0-alpha.1 [skip ci] ([ef473df](https://codeberg.org/goern/forgejo-mcp/commit/ef473df089351228342382548744de781ae98a7b))
- **release:** 1.1.0-alpha.2 [skip ci] ([458d31c](https://codeberg.org/goern/forgejo-mcp/commit/458d31cc15e29eb638381cdf619a7e2ddb275e45))
- **release:** 1.1.0-alpha.3 [skip ci] ([c53674e](https://codeberg.org/goern/forgejo-mcp/commit/c53674e4fa83b13f3b432889e31f0fbb0dcff876))

## 1.0.0 (2025-04-08)

### :sparkles: Features

- add stdio and sse MCP server ([38212fa](https://codeberg.org/goern/forgejo-mcp/commit/38212fabbe6b7a2e4cfe82d2bb8289c3a9ef97ed))
- consolidate T-016 implementation ([5afe6fd](https://codeberg.org/goern/forgejo-mcp/commit/5afe6fdc1b966114cc029a33d64e3fc46256965c))
- extend codeberg issue interface with validation and metadata support ([a426ec5](https://codeberg.org/goern/forgejo-mcp/commit/a426ec580cfe2dcb1f5062215f6aa2aac67ffdea))
- **issue-mgmt:** enhance getIssue command with extended metadata and caching ([13d183e](https://codeberg.org/goern/forgejo-mcp/commit/13d183e577994292c10eceb08f0d4cd7e14c31c5))
- **issue:** enhance getIssue with metadata and caching ([fcc8779](https://codeberg.org/goern/forgejo-mcp/commit/fcc8779c96f361bd9fa9a881297dc025c9004915))

### :bug: Fixes

- **build:** resolve TypeScript build errors ([4d125da](https://codeberg.org/goern/forgejo-mcp/commit/4d125da79db731f5c0ad7fa26b883e727c8c3143))
- improve error handling and rollback in CodebergService ([938bd54](https://codeberg.org/goern/forgejo-mcp/commit/938bd54f4595e1df4ede5b2eb235a0723556a734))

### :memo: Documentation

- add a screenshot of http server ([6ae7ebe](https://codeberg.org/goern/forgejo-mcp/commit/6ae7ebe1030d372646e38b59e4361d698ba16fc3))
- add development cost information ([6985d37](https://codeberg.org/goern/forgejo-mcp/commit/6985d37a4859bca5d6dca639affa631c94f0728a))
- **issue-mgmt:** analyze existing code structure and capabilities ([66a19df](https://codeberg.org/goern/forgejo-mcp/commit/66a19df1102fa38c974fef1344a99948ab8bbce7))
- update the feature planning ([8150b37](https://codeberg.org/goern/forgejo-mcp/commit/8150b37a220e4ad01d3c720734e0091e2f1889a1))
- update the README ([e2146be](https://codeberg.org/goern/forgejo-mcp/commit/e2146be2955ffd595821132b6e8113a3b6d7bd65))

### :zap: Refactor

- move TYPES to dedicated file to resolve circular dependency ([949400c](https://codeberg.org/goern/forgejo-mcp/commit/949400cff1bec330c47a49daaedbf0854fa2388b))

### :repeat: Chore

- the big rename ([a6168b8](https://codeberg.org/goern/forgejo-mcp/commit/a6168b879f880415769e5e519958ff90b4df7a29))
