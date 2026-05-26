## [2.25.1](https://codeberg.org/goern/forgejo-mcp/compare/v2.25.0...v2.25.1) (2026-05-26)

### :bug: Fixes

* 🐛 replace deprecated --output-signature with --bundle in cosign ([cee6465](https://codeberg.org/goern/forgejo-mcp/commit/cee64655e992fd17e77db81359b23777b6da76c5))
* 🔒️ repoint cosign-sign-release task at cosign-signing-key-artifacts ([7633666](https://codeberg.org/goern/forgejo-mcp/commit/763366673a31300ea0338178a3505b9e88edc637))
* 🔒️ repoint release-tools tekton tasks at cosign-signing-key-images ([d382534](https://codeberg.org/goern/forgejo-mcp/commit/d382534b5cf970a37732397e763e6aa0c58a066d))

### :repeat: Chore

* 🔧 default beads-dashboard status filter to open ([1003590](https://codeberg.org/goern/forgejo-mcp/commit/1003590e84fe7ee2eb92e8724552e5ab07eceee6))
* 🗂️ close forgejo-mcp-j52 after PR [#164](https://codeberg.org/goern/forgejo-mcp/issues/164) merged ([6a90151](https://codeberg.org/goern/forgejo-mcp/commit/6a90151541f28203bdb547204fa12ffccd7f889d))

## [2.25.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.24.2...v2.25.0) (2026-05-26)

### :sparkles: Features

* ✨ add CRT phosphor beads dashboard ([a620b17](https://codeberg.org/goern/forgejo-mcp/commit/a620b174c56fcfffe436ff7cb183e96e33837cda))
* ✨ add govulncheck + openspec to release-tools image ([c1316d8](https://codeberg.org/goern/forgejo-mcp/commit/c1316d8a7e60720337287e11ab77ec434e9586d1))
* ✨ render dep graph as subway map ([e6a6185](https://codeberg.org/goern/forgejo-mcp/commit/e6a61859559fdbc174975fa40f142b2bfabd119c))
* **ci:** 🚀 add release-tools image build + publish Tekton pipelines (iteration 2) ([5df98ba](https://codeberg.org/goern/forgejo-mcp/commit/5df98baca9a65f3b1dde12a816d760e9a71dcd87))
* **image:** 🐳 add release-tools OCI image source tree (iteration 1) ([240a544](https://codeberg.org/goern/forgejo-mcp/commit/240a544ed39b6889e3811d17bb4dea74d0224364))
* release-tools image source tree + build/publish Tekton pipelines ([#157](https://codeberg.org/goern/forgejo-mcp/issues/157)) ([e02cf56](https://codeberg.org/goern/forgejo-mcp/commit/e02cf5668f4f839645cf8f2b609210d08de71152))

### :bug: Fixes

* ✏️ buildah needs runAsUser 0 for chroot isolation with hi/go base ([41cdafc](https://codeberg.org/goern/forgejo-mcp/commit/41cdafc0bf060fe2cbb8be67041496c54abb62ff))
* ✏️ correct CEL macro files.any.exists → files.exists ([7706d17](https://codeberg.org/goern/forgejo-mcp/commit/7706d17411307a9a59a6c098370429b0ab69a363))
* 🐛 cosign sha256sum path — download to /tmp/cosign-linux-amd64 before verify ([8ffcd31](https://codeberg.org/goern/forgejo-mcp/commit/8ffcd31f83d89095eed980464e4ee94e4c4bda39))
* 🐛 filename mismatch in sha256sum + privileged SCC for buildah ([97ee636](https://codeberg.org/goern/forgejo-mcp/commit/97ee636e49f605e8d6338c8e9805e2eeaac024a6))
* 🐛 move release-tools PipelineRuns to .tekton/ root ([4649f98](https://codeberg.org/goern/forgejo-mcp/commit/4649f98c1bde32ad64c2887909b2418612faf031))
* 🐛 pipeline task fixes validated by run11 success ([0901bf0](https://codeberg.org/goern/forgejo-mcp/commit/0901bf0b9263462b3fee5d6a42d98db6987a7b95))
* 🐛 rootless buildah in build-image task for OpenShift SCC ([6a55f3f](https://codeberg.org/goern/forgejo-mcp/commit/6a55f3f02f1c584bb2b4c445f3356ebb5aceb9a8))
* 🔒️ address automated code review findings from PR [#157](https://codeberg.org/goern/forgejo-mcp/issues/157) ([d59b910](https://codeberg.org/goern/forgejo-mcp/commit/d59b9105e97405b7b0cd6116e42b4a8b7ae4f9ea))
* 🔒️ restore fail-closed cosign signing, remove bootstrap SKIP_SIGN ([8a282ca](https://codeberg.org/goern/forgejo-mcp/commit/8a282ca1e4ce59cb35c50f8b8573789457a5b914))
* 🔒️ SHA256-verify goreleaser+syft, drop piped install.sh ([da3cfb1](https://codeberg.org/goern/forgejo-mcp/commit/da3cfb12625190e908fc58480df47b6bde83063b))
* **ci:** 🔧 registry → codeberg.org/operate-first + cosign task bugs + bootstrap ([2f5dc51](https://codeberg.org/goern/forgejo-mcp/commit/2f5dc5123f3e378f695bc6acd3c26825566fec29))

### :memo: Documentation

* **openspec:** 📋 apply adversarial review patches to release-tools-image change ([ddbb5a0](https://codeberg.org/goern/forgejo-mcp/commit/ddbb5a091e527ba95555cd36cfa7bd9df819b4b8))
* **openspec:** 📋 propose release-tools-image change for op1st Tekton pipeline ([c814127](https://codeberg.org/goern/forgejo-mcp/commit/c81412792ef762cb9fa7de55784742a3c00a2ddc))

### :barber: Code-style

* 💄 calm CRT text styling for legibility ([3dec0b9](https://codeberg.org/goern/forgejo-mcp/commit/3dec0b9478e3911fddfb807c520f73395fa61eab)), closes [#e8f0e8](https://codeberg.org/goern/forgejo-mcp/issues/e8f0e8) [#0a0c0a](https://codeberg.org/goern/forgejo-mcp/issues/0a0c0a)
* 💄 drop oversized #BEADS hero from dashboard ([840f996](https://codeberg.org/goern/forgejo-mcp/commit/840f996de58c11feb95fef399f29043019d4eb11)), closes [#BEADS](https://codeberg.org/goern/forgejo-mcp/issues/BEADS)

### :zap: Refactor

* ♻️ rewrite .tekton/tasks/ to use release-tools image ([0dcb2e1](https://codeberg.org/goern/forgejo-mcp/commit/0dcb2e14e9f1986bb427f1b469d5b7a473ccea3d))

### :repeat: Chore

* 📋 archive release-tools-image openspec change + sync spec ([bd92d21](https://codeberg.org/goern/forgejo-mcp/commit/bd92d217e734e28557672705f9a772bc0a37d089))
* 🔧 set vendor label to 'Operate First, by #B4mad' ([0f1e649](https://codeberg.org/goern/forgejo-mcp/commit/0f1e6492ab5a732a433eca3b93d3b82b5a719a44))
* 🗂️ beads close forgejo-mcp-00o, 3h2, aps + session wrap ([1137f7e](https://codeberg.org/goern/forgejo-mcp/commit/1137f7eee788a60a44941765794130acb7b27bd4))
* 🗂️ beads jsonl claim forgejo-mcp-3h2 + forgejo-mcp-00o ([a7883a6](https://codeberg.org/goern/forgejo-mcp/commit/a7883a62eda9f163a7402cb4a30cf24bf0e14ead))
* 🗂️ beads jsonl close forgejo-mcp-p1p ([bbe36d6](https://codeberg.org/goern/forgejo-mcp/commit/bbe36d6b4eabf724013acb74112113f2f751eaaa))
* 🗂️ beads jsonl post-[#155](https://codeberg.org/goern/forgejo-mcp/issues/155) merge ([9c84d7f](https://codeberg.org/goern/forgejo-mcp/commit/9c84d7f2274fd202bc4bce931fddd96ecb392177))
* 🗂️ beads jsonl post-PR[#157](https://codeberg.org/goern/forgejo-mcp/issues/157) merge + close forgejo-mcp-1b4 ([ef5cf05](https://codeberg.org/goern/forgejo-mcp/commit/ef5cf057f3a2c4479598b7d2a876238e68a14dad))
* 🗂️ beads jsonl post-PR[#157](https://codeberg.org/goern/forgejo-mcp/issues/157) review fixes ([ef300ba](https://codeberg.org/goern/forgejo-mcp/commit/ef300baed3878aa651c4547ce0266f8170784dd0))
* 🗂️ beads jsonl post-tasks rewrite ([1ecb952](https://codeberg.org/goern/forgejo-mcp/commit/1ecb9527c1f0953ff7a8a06086436b01501fcb63))
* 🗂️ beads jsonl: capture release pipeline findings on P0 image bead ([082c71e](https://codeberg.org/goern/forgejo-mcp/commit/082c71e7c8806794e6fa9802791965e845ce5935))
* 🗂️ merge beads jsonl after [#161](https://codeberg.org/goern/forgejo-mcp/issues/161) ([8c1df9d](https://codeberg.org/goern/forgejo-mcp/commit/8c1df9dd8ed658bd7b0b51a7cf310f675f513782))

## [2.24.2](https://codeberg.org/goern/forgejo-mcp/compare/v2.24.1...v2.24.2) (2026-05-25)

### :bug: Fixes

* **ci:** 🐛 move Go cache out of workspace to avoid goreleaser dirty-tree check ([efcec22](https://codeberg.org/goern/forgejo-mcp/commit/efcec225df0b2122422faee2466b6cbc838e8767))

### :repeat: Chore

* 🗂️ beads jsonl post-[#154](https://codeberg.org/goern/forgejo-mcp/issues/154) merge ([9254e60](https://codeberg.org/goern/forgejo-mcp/commit/9254e60e6a7d974cc2f9b1bddafae24c6fceea37))

## [2.24.1](https://codeberg.org/goern/forgejo-mcp/compare/v2.24.0...v2.24.1) (2026-05-25)

### :bug: Fixes

* **ci:** 🐛 collapse goreleaser Task install+run into single step ([94b2c89](https://codeberg.org/goern/forgejo-mcp/commit/94b2c89a040499316dc1947fd7ca12538f7cffa3))

### :repeat: Chore

* 🗂️ beads jsonl post-[#153](https://codeberg.org/goern/forgejo-mcp/issues/153) merge ([1858753](https://codeberg.org/goern/forgejo-mcp/commit/1858753435480cf3726577f0ee5d8f8b9ca9d240))

## [2.24.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.23.1...v2.24.0) (2026-05-25)

### :sparkles: Features

* **ci:** 🚀 add Tekton release pipeline mirroring Forgejo Actions release ([cf3bddf](https://codeberg.org/goern/forgejo-mcp/commit/cf3bddf8c34f0f5be65e1464ebbda13c8a64927f))

### :bug: Fixes

* **ci:** 🔧 align release pipeline secret names with op1st-pipelines reality ([d2cc52b](https://codeberg.org/goern/forgejo-mcp/commit/d2cc52b433ab7f5c994e76045987ebf06e2f2e72))

### :memo: Documentation

* 📝 add Verifying Releases chapter with cosign instructions ([c1afa7a](https://codeberg.org/goern/forgejo-mcp/commit/c1afa7a3b2b45463aee87b3fd5c4ad9e6b762413))

### :repeat: Chore

* 🔥 remove stale secrets/ + point at op1st-emea-b4mad as normative cosign pub ([b63095f](https://codeberg.org/goern/forgejo-mcp/commit/b63095f21140eddef42f4a828b3cea0222d5e33e))
* 🗂️ beads jsonl post-[#150](https://codeberg.org/goern/forgejo-mcp/issues/150) merge ([641e8f1](https://codeberg.org/goern/forgejo-mcp/commit/641e8f1148ef72f5749ec11f2a8cba1d29e1d913))
* 🗂️ beads jsonl post-[#151](https://codeberg.org/goern/forgejo-mcp/issues/151) merge ([ca93489](https://codeberg.org/goern/forgejo-mcp/commit/ca93489319db0a220e576ae0e62f81a9045f7da5))
* 🗂️ beads jsonl post-[#152](https://codeberg.org/goern/forgejo-mcp/issues/152) merge ([ddc8206](https://codeberg.org/goern/forgejo-mcp/commit/ddc8206f5a4750a1f3da1b920f40ee3fe983e7b2))
* **ci:** 🔌 soft-disable Forgejo release workflow auto-trigger ([2cd6682](https://codeberg.org/goern/forgejo-mcp/commit/2cd6682ba7df3fbb3c285469230eb5964c28c5c6)), closes [#150](https://codeberg.org/goern/forgejo-mcp/issues/150)

## [2.23.1](https://codeberg.org/goern/forgejo-mcp/compare/v2.23.0...v2.23.1) (2026-05-25)

### :bug: Fixes

* 🚨 release: inject main.Version + use 'version' subcommand in smoke-test ([c74da84](https://codeberg.org/goern/forgejo-mcp/commit/c74da842681bb9b2cd73de5fb79fea71399ee67a)), closes [#172](https://codeberg.org/goern/forgejo-mcp/issues/172)

### :memo: Documentation

* 📝 credit byteflavour for v2.23.0 stateless-auth + NixOS docs ([298c6e4](https://codeberg.org/goern/forgejo-mcp/commit/298c6e48d91e0d65fbea434cc83ab981832d6ddc)), closes [#138](https://codeberg.org/goern/forgejo-mcp/issues/138) [#146](https://codeberg.org/goern/forgejo-mcp/issues/146)

### :repeat: Chore

* update beads jsonl ([a2756d4](https://codeberg.org/goern/forgejo-mcp/commit/a2756d440513d4a063dd0799a1dc0f3906794ce3))

## [2.23.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.22.0...v2.23.0) (2026-05-25)

### :sparkles: Features

* ✨ add 14 MCP tools for Forgejo releases and release attachments ([a41115b](https://codeberg.org/goern/forgejo-mcp/commit/a41115b9bbe442b036d84053c68349abb6fc9fb0))
* robust stateless auth with security fixes and improved tests ([4969ee5](https://codeberg.org/goern/forgejo-mcp/commit/4969ee588180ea5bbce501ea2262b0573e6f12f8))
* stateless per-request token handling for HTTP/SSE transports ([684844c](https://codeberg.org/goern/forgejo-mcp/commit/684844cbe6839ea68dc1f474bee185bc68682537))

### :bug: Fixes

* 🔒️ bump golang.org/x/crypto to v0.52.0 (GO-2026-5018) ([98b3cd5](https://codeberg.org/goern/forgejo-mcp/commit/98b3cd572aafd3cc940f72b56b2bc2baa7206064))
* 🔒️ bump x/net to v0.55.0 + jsonparser to v1.1.2 (govulncheck) ([e084bf7](https://codeberg.org/goern/forgejo-mcp/commit/e084bf70f7c1386c1b35ad933f5e1d2a698ee366))

### :memo: Documentation

* 📝 add demos/README.md index grouping demos by topic cluster ([621e664](https://codeberg.org/goern/forgejo-mcp/commit/621e6648d02b67d22801b9973507b41e8dec4e2d))
* 📝 add multi-tenant HTTP mode documentation and demo ([b53143f](https://codeberg.org/goern/forgejo-mcp/commit/b53143f18c45c547b2080f90ce5a2d51513753f8))
* 📝 add Radicle mirror clone instructions to README ([097516a](https://codeberg.org/goern/forgejo-mcp/commit/097516ade85c3b04c388efcce8708d7be5523261))
* 📝 add showboat demos for v2.22.0 (org labels, bounded responses) ([4baeb30](https://codeberg.org/goern/forgejo-mcp/commit/4baeb3030eebf85cf2beaada37c2ba19432689fe))
* 📝 archive 5 delivered openspec changes, track 2 unimplemented ([7eb8a34](https://codeberg.org/goern/forgejo-mcp/commit/7eb8a342346c40f95a9ebd8551e0f52e39469561)), closes [#129](https://codeberg.org/goern/forgejo-mcp/issues/129)
* 📝 archive add-releases-support, sync release-management spec ([1dc7a17](https://codeberg.org/goern/forgejo-mcp/commit/1dc7a17fb28b7c5dc47280abdcf609e7e8854328)), closes [#134](https://codeberg.org/goern/forgejo-mcp/issues/134)
* 📝 battle-test forgejo-action-code-review, resolve C4 spike ([48809bd](https://codeberg.org/goern/forgejo-mcp/commit/48809bd3a5cdf3880f30f01955c6cc7c4362a41d))
* 📝 link demos/ index from top-level README ([e55c259](https://codeberg.org/goern/forgejo-mcp/commit/e55c259ae5612adc83cad4ee68b1563430c2d2af))
* 📝 retrofit openspec for stateless-http-auth ([#137](https://codeberg.org/goern/forgejo-mcp/issues/137), PR [#138](https://codeberg.org/goern/forgejo-mcp/issues/138)) ([df15877](https://codeberg.org/goern/forgejo-mcp/commit/df15877777b177d9d237af26fab3c4a829ba61de))
* improve NixOS installation instructions ([9e29a13](https://codeberg.org/goern/forgejo-mcp/commit/9e29a130369f24b9f715857fb073e15f711f8dd5))

### :barber: Code-style

* 🎨 apply gofmt -w to operation/* and pkg/* ([34effc8](https://codeberg.org/goern/forgejo-mcp/commit/34effc817fb5f85bb0bbafd6919f1b5511ba625a))

### :repeat: CI

* 🔒️ add cosign-keygen.sh producing SOPS-encrypted k8s Secret ([42614b0](https://codeberg.org/goern/forgejo-mcp/commit/42614b036b9cb7eaf58ca97f2f5e8b55bfdd14ab))
* 🔒️ provision cosign signing material for release pipeline ([d3fcc3d](https://codeberg.org/goern/forgejo-mcp/commit/d3fcc3d2eb45d4e119e82e51ffba37d724888556))
* 🔧 gitleaks: allow placeholder tokens in demo docs ([ed5f419](https://codeberg.org/goern/forgejo-mcp/commit/ed5f4191954d3945d31d7d11850f0ed9c0974730))
* 🚀 add Forgejo Actions CI workflow with Go cache ([815a2d0](https://codeberg.org/goern/forgejo-mcp/commit/815a2d042674a47847645e19ae8a604150ede8f3))
* 🚀 add gitleaks scanning (Tekton + pre-commit) ([b936f8a](https://codeberg.org/goern/forgejo-mcp/commit/b936f8ab207ef1a56129ba45ff6890caf4ba0676))
* 🚀 add vet, gofmt-check, mod-tidy, lint, race, govulncheck to go-ci ([738d979](https://codeberg.org/goern/forgejo-mcp/commit/738d9795533c29c2da0814c5e5ad5c3d07269bff))
* 🚀 drop redundant PaC annotations on openspec-validate ([2b5721a](https://codeberg.org/goern/forgejo-mcp/commit/2b5721aaa114d2954121f737e1684ab8001d1504))
* 🚀 enable checksums and per-archive CycloneDX SBOMs in goreleaser ([74f601a](https://codeberg.org/goern/forgejo-mcp/commit/74f601a4eb90e191c51c36c66117ecf00bd87cbb))
* 🚀 release.yml: syft + cosign install, smoke-test, conditional sign ([c82fbee](https://codeberg.org/goern/forgejo-mcp/commit/c82fbeef424f0a4ae11eddff56344e0758e76991))

### :repeat: Chore

* 🔧 add Claude Code agent team infrastructure for multi-agent workflows ([3a73026](https://codeberg.org/goern/forgejo-mcp/commit/3a73026b164e83aa0438edb54c85962348d9ea44))
* 🔧 bd: claim 51l, link PR [#144](https://codeberg.org/goern/forgejo-mcp/issues/144) ([23be9fd](https://codeberg.org/goern/forgejo-mcp/commit/23be9fdaa5e708133705ae07989cb27f3acfc2ab))
* 🔧 bd: claim forgejo-mcp-9n2, link PR [#143](https://codeberg.org/goern/forgejo-mcp/issues/143) ([eed502b](https://codeberg.org/goern/forgejo-mcp/commit/eed502bab4d0d76a53b28c5654860af3ecf0a420))
* 🔧 bd: claim+close 02o (PR [#145](https://codeberg.org/goern/forgejo-mcp/issues/145) labeled Kind/Security) ([45bdadc](https://codeberg.org/goern/forgejo-mcp/commit/45bdadc2e0645b92ad350bc2209cfd7d9958a310))
* 🔧 bd: close 51l (PR [#144](https://codeberg.org/goern/forgejo-mcp/issues/144) merged) ([c0416e3](https://codeberg.org/goern/forgejo-mcp/commit/c0416e376d6655e61b08dab869a976a0a922adca))
* 🔧 bd: close 9n2, file e9i (ci.yml run [#147](https://codeberg.org/goern/forgejo-mcp/issues/147) failure) ([2ff1ddd](https://codeberg.org/goern/forgejo-mcp/commit/2ff1ddd2de645cc414d85e5f861a942f55271f5a))
* 🔧 bd: close dhd (PR [#147](https://codeberg.org/goern/forgejo-mcp/issues/147) merged), claim 1l5 RFC ([ebb70b3](https://codeberg.org/goern/forgejo-mcp/commit/ebb70b3dbc1c14414856238cad7c0a46fc3e0122))
* 🔧 bd: file + close forgejo-mcp-5x8 (PaC webhook fix) ([f305e33](https://codeberg.org/goern/forgejo-mcp/commit/f305e33fc295f034c502da14f67f3b82d03e6671))
* 🔧 bd: file C5 spike + C4 cleanup issues, link to forgejo-mcp-673 ([b6eb0a9](https://codeberg.org/goern/forgejo-mcp/commit/b6eb0a99777afd3e59bd9a08f98db23dff539133))
* 🔧 bd: file CI hardening epic (Steps 1–3 + follow-ups) ([f4cacf1](https://codeberg.org/goern/forgejo-mcp/commit/f4cacf1bd19ca15235bd1ae55f104b54d2e6b8f3))
* 🔧 bd: file dhd (gitleaks placeholder allowlist), claim, link PR [#147](https://codeberg.org/goern/forgejo-mcp/issues/147) ([fa2c5fc](https://codeberg.org/goern/forgejo-mcp/commit/fa2c5fc79eafc3482f770a01835d9da3017a9407))
* 🔧 reconcile .gitignore ([7063249](https://codeberg.org/goern/forgejo-mcp/commit/7063249513ad3cf75fe7b15cee436f3aef47046e))
* 🔧 switch Containerfile to Project Hummingbird base images ([3d8ade1](https://codeberg.org/goern/forgejo-mcp/commit/3d8ade1dd68626c7f99aebbec1307aace950d9fe))
* **deps:** update registry.access.redhat.com/hi/go docker tag to v1.26.3 ([a623431](https://codeberg.org/goern/forgejo-mcp/commit/a623431f3cd29e9da12d4a482b4e5d5026bf4d24))
* update beads jsonl ([bbbf84e](https://codeberg.org/goern/forgejo-mcp/commit/bbbf84ec266538b91b8e2bd375aa8ec5d36b99f9))

## [2.22.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.21.0...v2.22.0) (2026-05-12)

### :sparkles: Features

* add list_org_labels + merge org labels in list_repo_labels ([#130](https://codeberg.org/goern/forgejo-mcp/issues/130)) ([8862e83](https://codeberg.org/goern/forgejo-mcp/commit/8862e839138d33b5e1d80648e52b31617e0babc8)), closes [#125](https://codeberg.org/goern/forgejo-mcp/issues/125)
* bounded responses for get_pull_request_diff + get_file_content ([#131](https://codeberg.org/goern/forgejo-mcp/issues/131)) ([4cebe0b](https://codeberg.org/goern/forgejo-mcp/commit/4cebe0b0fd9cb31b0a1d935786cc96d9ed63a29b)), closes [#124](https://codeberg.org/goern/forgejo-mcp/issues/124)

### :bug: Fixes

* 🐛 send assignees array in update_issue ([6291213](https://codeberg.org/goern/forgejo-mcp/commit/62912136638dd7b2c6f0a6581a25da300d04b6b4)), closes [goern/forgejo-mcp#128](https://codeberg.org/goern/forgejo-mcp/issues/128)

### :memo: Documentation

* 📝 add openspec change for releases support ([#127](https://codeberg.org/goern/forgejo-mcp/issues/127)) ([d0bab23](https://codeberg.org/goern/forgejo-mcp/commit/d0bab236ba5ae03b6cf238a5f9f1f6fa8cde1fc6))
* 📝 add Purpose+Requirements headers to cli-mode spec ([23db899](https://codeberg.org/goern/forgejo-mcp/commit/23db89942529935bec24e8ae290ab608cc468195))
* 📝 add Purpose+Requirements headers to PR specs ([c8a7883](https://codeberg.org/goern/forgejo-mcp/commit/c8a7883f9424d99a52435aa13af8144d6e14cabb))
* 📝 add spec deltas to forgejo-action-code-review change ([a83a033](https://codeberg.org/goern/forgejo-mcp/commit/a83a03361d98eb743323c7cbcd2beeba614cc7e5))
* 📝 codify output-bounding rule for MCP tools ([1412cbe](https://codeberg.org/goern/forgejo-mcp/commit/1412cbe15ff9b464fc61cda451c85f0e117c48d8)), closes [#124](https://codeberg.org/goern/forgejo-mcp/issues/124) [#124](https://codeberg.org/goern/forgejo-mcp/issues/124)
* **extension:** address review feedback on [#118](https://codeberg.org/goern/forgejo-mcp/issues/118) ([a88c2a3](https://codeberg.org/goern/forgejo-mcp/commit/a88c2a325c337a15eb5e1001db4b4983085850f5))

### :zap: Refactor

* ♻️ rename openspec Tekton PipelineRuns ([85a8f4f](https://codeberg.org/goern/forgejo-mcp/commit/85a8f4fee401f60d16224aea673109bcd498a356))

### :repeat: CI

* 🚀 add openspec validate workflow ([1f6b256](https://codeberg.org/goern/forgejo-mcp/commit/1f6b256cfcb42e01e04d15e47aed81b774e6e554))
* 🚀 migrate CI from Forgejo Actions to op1st Tekton ([ae35a30](https://codeberg.org/goern/forgejo-mcp/commit/ae35a303a8c872b91f19fa6c6a6451777358aa6b))
* 🚀 migrate openspec validate from Forgejo Actions to op1st Tekton ([fbd1203](https://codeberg.org/goern/forgejo-mcp/commit/fbd1203e63e86fc3a1fb8dd09e32a0bd0f064e7b))

### :repeat: Chore

* 🔧 close beads forgejo-mcp-43k (Tekton CI migration) ([0be622d](https://codeberg.org/goern/forgejo-mcp/commit/0be622df177d909ff24d684cc234436dca87428c))
* 🔧 close forgejo-mcp-efo in beads tracker ([616c1ce](https://codeberg.org/goern/forgejo-mcp/commit/616c1ce1b4b3dec95967794d83a05b98c140ee82))
* 🔧 close forgejo-mcp-fdx (openspec Tekton migration) ([a385896](https://codeberg.org/goern/forgejo-mcp/commit/a385896d19447ceeddc24f80d080dd9da2ab3c6a))
* 🔧 ignore __pycache__ and add codeberg-issue-triage skill ([64b6a54](https://codeberg.org/goern/forgejo-mcp/commit/64b6a5443bdaeac126da45ef9c3958657829bffb))
* 🔧 link forgejo-mcp-43k bead to Codeberg [#133](https://codeberg.org/goern/forgejo-mcp/issues/133) ([6adda88](https://codeberg.org/goern/forgejo-mcp/commit/6adda88201be1693e2475c93cd64c67a0da8fbb8))
* 🔧 note PR [#130](https://codeberg.org/goern/forgejo-mcp/issues/130) merge in forgejo-mcp-7ch bead ([c0a8b20](https://codeberg.org/goern/forgejo-mcp/commit/c0a8b20d5465891d97f7f3b6340cb52156d7951b))
* 🔧 retest PaC after PAT scope fix ([59da82c](https://codeberg.org/goern/forgejo-mcp/commit/59da82c70ae7b89a8323150103e1f6bb7b71aff3))
* 🔧 retrigger PaC after PAT scope expansion ([a27df63](https://codeberg.org/goern/forgejo-mcp/commit/a27df637dcb5605333ad7c90b37b94ab436e0311))
* 🔧 retrigger PaC after webhook install ([2f496b4](https://codeberg.org/goern/forgejo-mcp/commit/2f496b499e3f1865e2e38cb6f8d48999c74cbbfb))
* 🔧 retrigger to confirm openspec PaC status flake is transient ([0e36348](https://codeberg.org/goern/forgejo-mcp/commit/0e36348640e9e0ba1a6ef312478c14d03581527e))
* 🔧 slim opsx to core 4 commands (apply/archive/explore/propose) ([7e718f5](https://codeberg.org/goern/forgejo-mcp/commit/7e718f509407d82e75326a25e034d52f538b46b5))
* 🔧 track add-bounded-text-responses follow-up in beads ([602ecaa](https://codeberg.org/goern/forgejo-mcp/commit/602ecaa070d131df8f07fbb054bd3c353ed91441)), closes [#124](https://codeberg.org/goern/forgejo-mcp/issues/124)
* 🔧 track follow-up bd issues from [#127](https://codeberg.org/goern/forgejo-mcp/issues/127) release-spec work ([0e53c51](https://codeberg.org/goern/forgejo-mcp/commit/0e53c511c64f8f6e48bc878d6a9f7ab02c7ea848)), closes [#129](https://codeberg.org/goern/forgejo-mcp/issues/129)

## [2.21.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.20.0...v2.21.0) (2026-05-07)

### :sparkles: Features

* **extension:** package as Claude Desktop Extension (.mcpb) ([998f92b](https://codeberg.org/goern/forgejo-mcp/commit/998f92b214ab8efb1a2bb7a1206b35aa4f3ca7e6))

### :memo: Documentation

* 📝 credit synath for .mcpb packaging and add claude-code agent ([8332e92](https://codeberg.org/goern/forgejo-mcp/commit/8332e92df862c27c3f111da13843e52c6bb3f5a6)), closes [#118](https://codeberg.org/goern/forgejo-mcp/issues/118) [116/#117](https://codeberg.org/116/forgejo-mcp/issues/117)

### :repeat: CI

* 🚀 build and attach .mcpb extension on tagged release ([fdc6305](https://codeberg.org/goern/forgejo-mcp/commit/fdc630558723bb53294af11b752f6e3ea5c6d0e4))

### :repeat: Chore

* 🔧 add mcpb and help make targets ([679d593](https://codeberg.org/goern/forgejo-mcp/commit/679d5939347dce4f9063a82acc1caee6fa310dca))
* **deps:** update golang:1.26-alpine docker digest to 91eda97 ([cb058c3](https://codeberg.org/goern/forgejo-mcp/commit/cb058c3c8faf8a2a52c6060903c198776f4a1624))
* **deps:** update golang:1.26-alpine docker digest to e58f92c ([13e52b5](https://codeberg.org/goern/forgejo-mcp/commit/13e52b56f30f8fb2e85467b5ee0e4b3c8630a8d2))

## [2.20.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.19.0...v2.20.0) (2026-05-06)

### :sparkles: Features

* add list_repo_contents and get_repo_tree MCP tools ([8c0759d](https://codeberg.org/goern/forgejo-mcp/commit/8c0759de5b1aed5ce0482374dcb9fb80ba7d4720))
* get_file_content returns plain text by default ([d76be68](https://codeberg.org/goern/forgejo-mcp/commit/d76be6886671e4f086d3c2965087ddc3a0c34b12))
* name binary-file case in get_file_content description ([c2c33de](https://codeberg.org/goern/forgejo-mcp/commit/c2c33de1846354e078d7478872d6d05bb1076ef4)), closes [#116](https://codeberg.org/goern/forgejo-mcp/issues/116)

### :memo: Documentation

* 📝 add BrilliantKahn to contributors with first-OSS-PR note ([7f214f5](https://codeberg.org/goern/forgejo-mcp/commit/7f214f503230651f0b95d126c6c6f8ab6942f742))

## [2.19.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.18.0...v2.19.0) (2026-05-02)

### :sparkles: Features

* add issue & comment attachment tools ([#109](https://codeberg.org/goern/forgejo-mcp/issues/109)) ([d0cfd66](https://codeberg.org/goern/forgejo-mcp/commit/d0cfd6691dedf6e1cf4ab37290630043b9c07655))

### :bug: Fixes

* check merged bool in MergePullRequestFn ([#113](https://codeberg.org/goern/forgejo-mcp/issues/113)) ([5326fb4](https://codeberg.org/goern/forgejo-mcp/commit/5326fb41928d0481289108ee2994b5ef1f9e237c))
* use ServerVersion for connection check instead of GetMyUserInfo ([#112](https://codeberg.org/goern/forgejo-mcp/issues/112)) ([b427041](https://codeberg.org/goern/forgejo-mcp/commit/b42704109733a5a812da8635d16ca41f09893cd7))

### :memo: Documentation

* 📝 add synath and heathen711 to contributors ([db0d925](https://codeberg.org/goern/forgejo-mcp/commit/db0d925f5f95dac48cde5724b742452ae5b47a8f)), closes [#112](https://codeberg.org/goern/forgejo-mcp/issues/112) [#113](https://codeberg.org/goern/forgejo-mcp/issues/113) [#106](https://codeberg.org/goern/forgejo-mcp/issues/106)
* 📝 amend issue-attachments spec: 1 MiB cap, always include download URL ([91bc2dd](https://codeberg.org/goern/forgejo-mcp/commit/91bc2dd30afad1455cb7122d206def990c6e43e2)), closes [#2](https://codeberg.org/goern/forgejo-mcp/issues/2)

## [2.18.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.17.0...v2.18.0) (2026-04-21)

### :sparkles: Features

* ✨ add issue/PR time tracking and stopwatch tools ([e28cd91](https://codeberg.org/goern/forgejo-mcp/commit/e28cd91262b06a19193b0bc56eae8b83e7d2227d))

### :memo: Documentation

* 📝 add issue-attachment spec and beads issue tracking ([ae2c981](https://codeberg.org/goern/forgejo-mcp/commit/ae2c9813add8644b48e6cb108256799a9f2466c7)), closes [#106](https://codeberg.org/goern/forgejo-mcp/issues/106) [#98](https://codeberg.org/goern/forgejo-mcp/issues/98)
* 📝 add issue/PR time tracking spec ([3d9fd1a](https://codeberg.org/goern/forgejo-mcp/commit/3d9fd1af5008e9ed8162019e0940bacb90c70c96))

### :repeat: Chore

* **deps:** update golang:1.26-alpine docker digest to 1fb7391 ([f5c89b5](https://codeberg.org/goern/forgejo-mcp/commit/f5c89b529fbb22c94b93c422c18ec608da2ac78e))
* **deps:** update golang:1.26-alpine docker digest to 27f8293 ([a24ccc7](https://codeberg.org/goern/forgejo-mcp/commit/a24ccc7b6d0e6902b2afb238be7d2dbd0b6678f0))
* **deps:** update golang:1.26-alpine docker digest to c2a1f7b ([de73b9e](https://codeberg.org/goern/forgejo-mcp/commit/de73b9e2b28e2af2a6f7e2e4d98baca4917ece71))
* **deps:** update golang:1.26-alpine docker digest to f853308 ([9c6540c](https://codeberg.org/goern/forgejo-mcp/commit/9c6540c207ce4fb9c0d70f9b698bf22fe273dcae))

## [2.17.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.16.0...v2.17.0) (2026-03-27)

### :sparkles: Features

* ✨ add streamable HTTP transport support ([#99](https://codeberg.org/goern/forgejo-mcp/issues/99)) ([cee3993](https://codeberg.org/goern/forgejo-mcp/commit/cee39936c04e80baf59143296e4643f27b4b16d1))

### :memo: Documentation

* 📝 update contributors with Vokuar and janbaer ([885e9c0](https://codeberg.org/goern/forgejo-mcp/commit/885e9c0f7c351547b5e2cbef702b6b9354e88189))

## [2.16.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.15.1...v2.16.0) (2026-03-20)

### :sparkles: Features

* ✨ add organization management tools (CRUD, membership, teams) ([2c4c02c](https://codeberg.org/goern/forgejo-mcp/commit/2c4c02c332101197a8aee1603162db8841d95e0f))
* add remove_issue_labels tool ([be758d4](https://codeberg.org/goern/forgejo-mcp/commit/be758d43739eae4d8a20e66c0389de7d932fbf36)), closes [ardi/ardi-crm#96](https://codeberg.org/ardi/ardi-crm/issues/96)

### :memo: Documentation

* ✨ add OpenSpec artifacts for organization management feature ([2d540a5](https://codeberg.org/goern/forgejo-mcp/commit/2d540a5b60d5e815bd6b0b823ed545790aeccd77)), closes [#92](https://codeberg.org/goern/forgejo-mcp/issues/92)
* 📝 add showboat demo for issue label management tools ([004a2c6](https://codeberg.org/goern/forgejo-mcp/commit/004a2c611f21938afa5b96558599554ed6558a22))
* 📝 add showboat demo for organization management tools ([cf407fd](https://codeberg.org/goern/forgejo-mcp/commit/cf407fdf27f5b0618077732c9a173d88fec8d83c))
* 📝 update contributors with ignasgil and dmikushin ([8e93b89](https://codeberg.org/goern/forgejo-mcp/commit/8e93b89274870843451f908d5066ee1def58a988))

### :repeat: Chore

* 🔥 remove obsolete smithery.yaml and mcp-settings-sample.json ([b040259](https://codeberg.org/goern/forgejo-mcp/commit/b040259f8f7eabeeba2b048ab1e022ef5c0fd949))

## [2.15.1](https://codeberg.org/goern/forgejo-mcp/compare/v2.15.0...v2.15.1) (2026-03-14)

### :memo: Documentation

* ✨ add missing contributors (Ronmi Ren, jiriks74, th, opencode) ([6fbb827](https://codeberg.org/goern/forgejo-mcp/commit/6fbb827663ebb467ea1341e7ffe10960e627e75f)), closes [#51](https://codeberg.org/goern/forgejo-mcp/issues/51)

### :repeat: Chore

* bump github.com/mark3labs/mcp-go from v0.43.2 to v0.44.0 ([4a18f7f](https://codeberg.org/goern/forgejo-mcp/commit/4a18f7f8bd8325565453711a6480d7b9f3629976))

## [2.15.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.14.0...v2.15.0) (2026-03-11)

### :sparkles: Features

* add unified-notifications skill for GitHub and Codeberg ([39f3021](https://codeberg.org/goern/forgejo-mcp/commit/39f302107677a650ad6ec436f856759fe3c93c2b))
* add user agent configuration support ([d3cb629](https://codeberg.org/goern/forgejo-mcp/commit/d3cb629db7a6fde31cc97a14e54703daa838adb4))
* trigger 2.15.0 release ([9768a1d](https://codeberg.org/goern/forgejo-mcp/commit/9768a1d54ca5c22b44929a98deaecbcaa6642526))

### :memo: Documentation

* extend Contributors section with community contributors (intercom-fep.2.1) ([df8e7b9](https://codeberg.org/goern/forgejo-mcp/commit/df8e7b9698968ae15b65a393bb20e8ca448f8613))
* update contributors with new members, fix links, and add highlights ([d4c9ebb](https://codeberg.org/goern/forgejo-mcp/commit/d4c9ebb5032b284557ecc303cad93c03eb1e9ee5))

### :repeat: Chore

* **release:** 2.14.0 ([26e20b3](https://codeberg.org/goern/forgejo-mcp/commit/26e20b3626c29b4fdd897de760300073fb373e3a))

## [2.14.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.13.0...v2.14.0) (2026-03-11)

### :sparkles: Features

* add unified-notifications skill for GitHub and Codeberg ([39f3021](https://codeberg.org/goern/forgejo-mcp/commit/39f302107677a650ad6ec436f856759fe3c93c2b))
* add user agent configuration support ([d3cb629](https://codeberg.org/goern/forgejo-mcp/commit/d3cb629db7a6fde31cc97a14e54703daa838adb4))
* **notifications:** implement complete notification API ([78916f6](https://codeberg.org/goern/forgejo-mcp/commit/78916f6ef1f897b2597f45a91263fc5bf59f069a))
* trigger 2.15.0 release ([9768a1d](https://codeberg.org/goern/forgejo-mcp/commit/9768a1d54ca5c22b44929a98deaecbcaa6642526))

### :bug: Fixes

* **notifications:** address PR [#86](https://codeberg.org/goern/forgejo-mcp/issues/86) review comments ([2e5d62b](https://codeberg.org/goern/forgejo-mcp/commit/2e5d62bdbc63f69fa5cb294ada554d1f7179ce4d))

### :memo: Documentation

* add Contributors section to README (intercom-fep.1) ([17304d5](https://codeberg.org/goern/forgejo-mcp/commit/17304d5e766ba8adfee9edd668ea6f30e1efcb24))
* extend Contributors section with community contributors (intercom-fep.2.1) ([df8e7b9](https://codeberg.org/goern/forgejo-mcp/commit/df8e7b9698968ae15b65a393bb20e8ca448f8613))
* update contributors with new members, fix links, and add highlights ([d4c9ebb](https://codeberg.org/goern/forgejo-mcp/commit/d4c9ebb5032b284557ecc303cad93c03eb1e9ee5))

### :repeat: Chore

* **release:** 2.14.0 ([8d59fdf](https://codeberg.org/goern/forgejo-mcp/commit/8d59fdf0be545b88a3330878576e6ba2888c7e26))

## [2.14.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.13.0...v2.14.0) (2026-03-08)

### :sparkles: Features

* **notifications:** implement complete notification API ([78916f6](https://codeberg.org/goern/forgejo-mcp/commit/78916f6ef1f897b2597f45a91263fc5bf59f069a))

### :bug: Fixes

* **notifications:** address PR [#86](https://codeberg.org/goern/forgejo-mcp/issues/86) review comments ([2e5d62b](https://codeberg.org/goern/forgejo-mcp/commit/2e5d62bdbc63f69fa5cb294ada554d1f7179ce4d))

### :memo: Documentation

* add Contributors section to README (intercom-fep.1) ([6819590](https://codeberg.org/goern/forgejo-mcp/commit/681959021ba4f8b979a3ae206a7d7bfa3c9569ee))

## [2.13.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.12.0...v2.13.0) (2026-03-07)

### :sparkles: Features

* add check_notifications tool ([f075a45](https://codeberg.org/goern/forgejo-mcp/commit/f075a45467668cb634002b8e47c22d66fde731b2))
* add list_repo_milestones + list_repo_labels MCP tools (closes [#80](https://codeberg.org/goern/forgejo-mcp/issues/80)) ([8a87af1](https://codeberg.org/goern/forgejo-mcp/commit/8a87af15b90d91038f63a1e5eb03d021788959b9))

### :repeat: Chore

* **deps:** update golang:1.26-alpine docker digest to 2389ebf ([ebe684e](https://codeberg.org/goern/forgejo-mcp/commit/ebe684ebcc03859bfeac932a22cde644e4a0523d))

## [Unreleased]

### :sparkles: Features

* ✨ add `list_repo_milestones` and `list_repo_labels` MCP tools for milestone/label discovery ([#80](https://codeberg.org/goern/forgejo-mcp/issues/80))

## [2.12.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.11.0...v2.12.0) (2026-03-01)

### :sparkles: Features

* ✨ add code-review skill and OpenSpec specs/tasks ([d39cb01](https://codeberg.org/goern/forgejo-mcp/commit/d39cb013813171142868f6e0ff36576554633b7a))
* ✨ add design doc for Forgejo code review skill ([697e207](https://codeberg.org/goern/forgejo-mcp/commit/697e20794198950d08c2cd007e79adeace5e556e))
* ✨ add OpenSpec changes for Forgejo code review integration ([03c7dc0](https://codeberg.org/goern/forgejo-mcp/commit/03c7dc09e15cb523f62d7a92b52760a6bafa4b13))
* ✨ add usage tracking to code-review skill ([41307db](https://codeberg.org/goern/forgejo-mcp/commit/41307db405cd2d9e26fd5a17b6754a5e5e094617))

### :bug: Fixes

* ✨ add list_pull_request_files/get_pull_request_diff tools, rewrite code-review skill ([19f2034](https://codeberg.org/goern/forgejo-mcp/commit/19f2034bcf6eda4882807d647e3fe9960637da58))
* 🔒️ prevent shell injection in code-review CLI fallback ([8b255f9](https://codeberg.org/goern/forgejo-mcp/commit/8b255f90b370ffaccc7ab206fd0c41ac4196c4db))
* nil pointer deref on resp.StatusCode and flag.Parse() in init() ([dfdb995](https://codeberg.org/goern/forgejo-mcp/commit/dfdb9959fc5102783ecfb5fa75a436bdb749ce67)), closes [#76](https://codeberg.org/goern/forgejo-mcp/issues/76)
* some local leftovers merged ([44ac633](https://codeberg.org/goern/forgejo-mcp/commit/44ac633eb353b8f2af8eba3323174dbf27c7ec85))

### :memo: Documentation

* 📝 document go install known issue and link to [#67](https://codeberg.org/goern/forgejo-mcp/issues/67) ([4388b47](https://codeberg.org/goern/forgejo-mcp/commit/4388b47ed3758be6f30434d594e75008a65a25f4))

### :white_check_mark: Tests

* add race condition reproducer for [#76](https://codeberg.org/goern/forgejo-mcp/issues/76) ([afddf59](https://codeberg.org/goern/forgejo-mcp/commit/afddf59cc451ad7e160437360ce16a86249123c5))

## [2.11.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.10.1...v2.11.0) (2026-02-18)

### :sparkles: Features

* ✨ add --version flag (GNU standard) alongside version subcommand ([6255653](https://codeberg.org/goern/forgejo-mcp/commit/6255653c2ef3b8216df6441b527e284d8a364d7d)), closes [#73](https://codeberg.org/goern/forgejo-mcp/issues/73)
* ✨ add Actions support (dispatch_workflow, list_workflow_runs, get_workflow_run) ([4fea365](https://codeberg.org/goern/forgejo-mcp/commit/4fea365203c90e832925f6e1c30179f47d5f5ce3)), closes [#103](https://codeberg.org/goern/forgejo-mcp/issues/103) [#60](https://codeberg.org/goern/forgejo-mcp/issues/60)

### :bug: Fixes

* 🐛 register actions domain in CLI tool listing ([ebe97a0](https://codeberg.org/goern/forgejo-mcp/commit/ebe97a0096c29788aef1474610613491f4071890))

### :memo: Documentation

* 📝 add Actions tools to README with list_workflow_runs examples ([af4636e](https://codeberg.org/goern/forgejo-mcp/commit/af4636e846868071234fa09525bf022f774be9a2))

## [2.10.1](https://codeberg.org/goern/forgejo-mcp/compare/v2.10.0...v2.10.1) (2026-02-11)

### :bug: Fixes

* 🐛 base64-encode content in create_file and update_file ([#72](https://codeberg.org/goern/forgejo-mcp/issues/72)) ([593eeb8](https://codeberg.org/goern/forgejo-mcp/commit/593eeb832fbefe5d2a4d4c9fb66c1dd1a8529fc5))

### :repeat: Chore

* **deps:** update golang docker tag to v1.26 ([85dca17](https://codeberg.org/goern/forgejo-mcp/commit/85dca175d80f5c7275b732197c11064bbce5ddde))

## [2.10.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.9.1...v2.10.0) (2026-02-10)

### :sparkles: Features

* ✨ add macOS (darwin) release assets ([b63aebc](https://codeberg.org/goern/forgejo-mcp/commit/b63aebcac21b5674a1dfde36e42a75cf4b5d6ce5)), closes [#70](https://codeberg.org/goern/forgejo-mcp/issues/70)

### :memo: Documentation

* add Arch Linux AUR installation options ([94a4370](https://codeberg.org/goern/forgejo-mcp/commit/94a437099efb476e27bd3c9cbde9233f3c722f9d))

## [2.9.1](https://codeberg.org/goern/forgejo-mcp/compare/v2.9.0...v2.9.1) (2026-02-06)

### :bug: Fixes

* 🔧 remove no-op replace directive from go.mod ([5629f7b](https://codeberg.org/goern/forgejo-mcp/commit/5629f7bfcac7ca87d83f6c4c41b1e99581c6a04d)), closes [#67](https://codeberg.org/goern/forgejo-mcp/issues/67)

### :repeat: Chore

* 📦 archive add-global-cli-mode change, sync cli-mode spec ([d505c99](https://codeberg.org/goern/forgejo-mcp/commit/d505c99e4aea76bc52f8821153805060599629ab))
* **deps:** update golang:1.25-alpine docker digest to f6751d8 ([36c3d6a](https://codeberg.org/goern/forgejo-mcp/commit/36c3d6aeb2a2fefd8fdf20f6d9d7270b57bdb844))

## [2.9.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.8.1...v2.9.0) (2026-02-06)

### :sparkles: Features

* ✨ add --cli mode for direct tool invocation ([c08e311](https://codeberg.org/goern/forgejo-mcp/commit/c08e311e69a5f56d6c5625de0534426bfd7ac516))

### :bug: Fixes

* 🐛 release workflow checkout URL and skip-ci suppression ([351e8b7](https://codeberg.org/goern/forgejo-mcp/commit/351e8b7d05663cea472a85b77273b96990d35c80))

### :memo: Documentation

* 📝 add CLI mode proposal (OpenSpec change) ([641b04a](https://codeberg.org/goern/forgejo-mcp/commit/641b04aed71ef06eb0658e6dcfd40f7ec59e9420))
* 📝 add CLI mode section to README ([5e54b1c](https://codeberg.org/goern/forgejo-mcp/commit/5e54b1c5ea61eb115842fcc8f4a31fa169e7c95f))

### :repeat: Chore

* 🔧 switch to redbeard's forgejo-sdk fork ([14cb733](https://codeberg.org/goern/forgejo-mcp/commit/14cb7338ebf2ad1d7ef0f65b9245f88d1bdf9b88))

## [2.8.1](https://codeberg.org/goern/forgejo-mcp/compare/v2.8.0...v2.8.1) (2026-02-06)

### :repeat: Chore

* 📦 archive merge-pull-request openspec change ([194d830](https://codeberg.org/goern/forgejo-mcp/commit/194d8303c4cdb2f8eb9a6bb8bda79204c9a79118))
* **deps:** update golang:1.25-alpine docker digest to f4622e3 ([726e033](https://codeberg.org/goern/forgejo-mcp/commit/726e03365abf1e847b7d48c8c34ffe92cc2e217a))

## [2.8.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.7.0...v2.8.0) (2026-01-31)

### :sparkles: Features

* ✨ add merge_pull_request MCP tool ([e416a4a](https://codeberg.org/goern/forgejo-mcp/commit/e416a4a4084747b5ef60dd6ff5a891d5453401cd)), closes [#54](https://codeberg.org/goern/forgejo-mcp/issues/54)

### :repeat: Chore

* 📦 archive pr-review-tool openspec change ([7b3232a](https://codeberg.org/goern/forgejo-mcp/commit/7b3232a50ccbf0cd34495a1a26f6ca899ebb500c))

## [2.7.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.6.1...v2.7.0) (2026-01-30)

### :sparkles: Features

* ✨ add write-side PR review tools ([578643c](https://codeberg.org/goern/forgejo-mcp/commit/578643ccf4e6a9875d8f24d2870de2ab88d45619)), closes [#59](https://codeberg.org/goern/forgejo-mcp/issues/59)

## [2.6.1](https://codeberg.org/goern/forgejo-mcp/compare/v2.6.0...v2.6.1) (2026-01-30)

### :bug: Fixes

* 🐛 show module version when installed via `go install` ([c80dbfd](https://codeberg.org/goern/forgejo-mcp/commit/c80dbfd55a849bcd6c3ae38d838f9719d25199a3))

## [2.6.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.5.1...v2.6.0) (2026-01-30)

### :sparkles: Features

* ✨ add version subcommand with build-time git info ([433446b](https://codeberg.org/goern/forgejo-mcp/commit/433446bfb61d95c57602341189aaf878cccaa74f))

### :bug: Fixes

* 🐛 use SemVer-compliant version format (-dev+commit) ([3b2bbbb](https://codeberg.org/goern/forgejo-mcp/commit/3b2bbbbef5599d8ab73cdcc566fd3473449ab2cb))

## [2.5.1](https://codeberg.org/goern/forgejo-mcp/compare/v2.5.0...v2.5.1) (2026-01-30)

### :bug: Fixes

* 🐛 adapt to mcp-go v0.43.2 breaking change in CallToolParams.Arguments ([a896e70](https://codeberg.org/goern/forgejo-mcp/commit/a896e70081067bf3261dc8e229a4292366204ecf))

### :repeat: Chore

* 📝 add OpenSpec proposal and design for PR review tools ([#59](https://codeberg.org/goern/forgejo-mcp/issues/59)) ([9475c00](https://codeberg.org/goern/forgejo-mcp/commit/9475c002ad402648d94a5b7e1e1a41c018b6ae1f))
* 🚀 add OpenSpec workflow, beads issue tracker, and merge PR proposal ([b469158](https://codeberg.org/goern/forgejo-mcp/commit/b46915894402e1667b6a04fc7229af5efda03998))
* 🚀 more claude config ([f420446](https://codeberg.org/goern/forgejo-mcp/commit/f420446dd4d1a2821b74e38437a1713475555933))
* **deps:** update alpine:edge docker digest to 9a341ff ([63afe57](https://codeberg.org/goern/forgejo-mcp/commit/63afe574fd80d19dfc82b5bef97ad51db9be5308))
* **deps:** update golang:1.25-alpine docker digest to 660f0b8 ([a799cb8](https://codeberg.org/goern/forgejo-mcp/commit/a799cb8e619d017582a5b5025cde09b335f19087))
* **deps:** update golang:1.25-alpine docker digest to 98e6cff ([5379f45](https://codeberg.org/goern/forgejo-mcp/commit/5379f455a1b066729da44faa7cf8e7ab2958b4fa))
* **deps:** update golang:1.25-alpine docker digest to 9f7db8d ([3c5b5cb](https://codeberg.org/goern/forgejo-mcp/commit/3c5b5cba298eb9ae941d2b260ce40a355291acbb))
* **deps:** update golang:1.25-alpine docker digest to d9b2e14 ([e798735](https://codeberg.org/goern/forgejo-mcp/commit/e798735c88c8865433d0fdaccdb5ec4edf24f963))
* **deps:** update golang:1.25-alpine docker digest to e689855 ([1d1c35b](https://codeberg.org/goern/forgejo-mcp/commit/1d1c35bb60de67d7eaea3600c0ec347944f33fc3))

## [2.5.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.4.2...v2.5.0) (2026-01-06)

### :sparkles: Features

* **pull:** add pull request reviews and review comments support ([f2ff5be](https://codeberg.org/goern/forgejo-mcp/commit/f2ff5beef584e3ffb5d2e45d07d0822fab511487))

### :memo: Documentation

* update README with new pull request review tools ([f0c218f](https://codeberg.org/goern/forgejo-mcp/commit/f0c218f38aa2ef1e4a85818b2faeb92f502db0ac))

## [2.4.2](https://codeberg.org/goern/forgejo-mcp/compare/v2.4.1...v2.4.2) (2025-12-30)

### :bug: Fixes

* add build tag to wiki package for Nix build compatibility ([7b7536a](https://codeberg.org/goern/forgejo-mcp/commit/7b7536a10d813cc9fac126b80b30508c1bf72d05)), closes [#47](https://codeberg.org/goern/forgejo-mcp/issues/47)

### :memo: Documentation

* restructure documentation for users and developers ([7e527e0](https://codeberg.org/goern/forgejo-mcp/commit/7e527e0acfe5dc7dd36079a1dae9a898139f6aa0))
* simplify AGENTS.md and reference DEVELOPER.md ([e294aa3](https://codeberg.org/goern/forgejo-mcp/commit/e294aa316276a7af2036a91f61a128b914d50a1a))

## [2.4.1](https://codeberg.org/goern/forgejo-mcp/compare/v2.4.0...v2.4.1) (2025-12-29)

### :bug: Fixes

* add /v2 suffix to module path for go install compatibility ([2e2604d](https://codeberg.org/goern/forgejo-mcp/commit/2e2604dd73465d57b05682775b320b544245d98e))

## [2.4.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.3.0...v2.4.0) (2025-12-29)

### :sparkles: Features

* enable `go install` support ([cf9f6d3](https://codeberg.org/goern/forgejo-mcp/commit/cf9f6d3a5bba2f1da6970123896a1198ebef31ae)), closes [#49](https://codeberg.org/goern/forgejo-mcp/issues/49)

### :repeat: Chore

* reconfig gitignore ([4e657b2](https://codeberg.org/goern/forgejo-mcp/commit/4e657b2e31e6261897b9d5bd43a47a9a565b83f6))

## [2.3.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.2.0...v2.3.0) (2025-12-29)

### :sparkles: Features

* **issue:** add comment management operations ([708392b](https://codeberg.org/goern/forgejo-mcp/commit/708392b0f4293cc73dcbecdcab90bc4bba9b656a))
* **pull:** add update_pull_request tool ([81fb6ac](https://codeberg.org/goern/forgejo-mcp/commit/81fb6ac83147c89245ab4400395f78b975dae1d1))

### :bug: Fixes

* release configuration ([193029c](https://codeberg.org/goern/forgejo-mcp/commit/193029cf9ff95a0c1718b338143d7730c6a3e916))

### :memo: Documentation

* add projects support plan and AI agent configuration ([c98ee59](https://codeberg.org/goern/forgejo-mcp/commit/c98ee59c29a67d07a81f44e8687e8143e4bbb01f)), closes [#42](https://codeberg.org/goern/forgejo-mcp/issues/42)
* add wiki support implementation plan ([79d13bb](https://codeberg.org/goern/forgejo-mcp/commit/79d13bbb5458b3456016f123c196fa480a0a8cc7))

### :zap: Refactor

* **mcp:** optimize tool definitions for token efficiency ([06d0ece](https://codeberg.org/goern/forgejo-mcp/commit/06d0ece1103b177dff6ef49115655b0e370eb376))

### :repeat: Chore

* **deps:** update alpine:edge docker digest to ea71a03 ([be13f1f](https://codeberg.org/goern/forgejo-mcp/commit/be13f1fa9b38c129f8d68650b8e5cca623bc595f))
* **deps:** update golang:1.25-alpine docker digest to 06cdd34 ([f0df0a3](https://codeberg.org/goern/forgejo-mcp/commit/f0df0a371d3e734e2a6a71916570b763a74fcc67))
* **deps:** update golang:1.25-alpine docker digest to 182059d ([c436e8a](https://codeberg.org/goern/forgejo-mcp/commit/c436e8abfc030bb2ff0eb7c6746eb0c465894949))
* **deps:** update golang:1.25-alpine docker digest to 2611181 ([ad1add3](https://codeberg.org/goern/forgejo-mcp/commit/ad1add302b65ea57fc36c8eec2bc4a78957100de))
* **deps:** update golang:1.25-alpine docker digest to 352f1ef ([48c9080](https://codeberg.org/goern/forgejo-mcp/commit/48c90805ef617f9f2e8f85f241bab88844e29c22))
* **deps:** update golang:1.25-alpine docker digest to 3587db7 ([1999fd6](https://codeberg.org/goern/forgejo-mcp/commit/1999fd6566c212bc5d728c99a58518c31974f302))
* **deps:** update golang:1.25-alpine docker digest to 6104e2b ([4f5225b](https://codeberg.org/goern/forgejo-mcp/commit/4f5225b091e40dc715f8b216cc97395060b17760))
* **deps:** update golang:1.25-alpine docker digest to 7256733 ([da02a42](https://codeberg.org/goern/forgejo-mcp/commit/da02a426062684df4167897a5e0f6fc36328d6fe))
* **deps:** update golang:1.25-alpine docker digest to 8280f72 ([1353798](https://codeberg.org/goern/forgejo-mcp/commit/13537983dad77d5756e160e3b62af7ccf2b2f91c))
* **deps:** update golang:1.25-alpine docker digest to 8b6b77a ([cbbaeb9](https://codeberg.org/goern/forgejo-mcp/commit/cbbaeb95139ee185a4e01a5132db3246db9a22a5))
* **deps:** update golang:1.25-alpine docker digest to a86c313 ([dceb9f5](https://codeberg.org/goern/forgejo-mcp/commit/dceb9f56e6a95e577eeeb0104338080b348fc910))
* **deps:** update golang:1.25-alpine docker digest to ac09a5f ([44ab927](https://codeberg.org/goern/forgejo-mcp/commit/44ab927a296ad03e9823c12faa67a8fb695c0631))
* **deps:** update golang:1.25-alpine docker digest to aee43c3 ([7ec667b](https://codeberg.org/goern/forgejo-mcp/commit/7ec667bf38c7b59df3effabfff823466b53c35e7))
* **deps:** update golang:1.25-alpine docker digest to d3f0cf7 ([8d32231](https://codeberg.org/goern/forgejo-mcp/commit/8d32231338c0c9fc7370c8db8190a257c5f3a76a))
* **deps:** update golang:1.25-alpine docker digest to ecb8038 ([c6ec90d](https://codeberg.org/goern/forgejo-mcp/commit/c6ec90d023a57e56ad56b2a48cc7c2d1d256e1de))
* remove unused roo configuration ([4121a50](https://codeberg.org/goern/forgejo-mcp/commit/4121a50f9358e4ea3b5bb3a57e9c438fc7957769))

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
