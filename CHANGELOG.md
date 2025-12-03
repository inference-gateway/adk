# Changelog

All notable changes to this project will be documented in this file.

## [0.16.0](https://github.com/inference-gateway/adk/compare/v0.15.3...v0.16.0) (2025-12-03)

### ✨ Features

* **callbacks:** Integrate callback hooks into agent execution flow ([#118](https://github.com/inference-gateway/adk/issues/118)) ([b69b34b](https://github.com/inference-gateway/adk/commit/b69b34b21994277bb2c8401341664a017bac3237)), closes [#94](https://github.com/inference-gateway/adk/issues/94) [#71](https://github.com/inference-gateway/adk/issues/71)

### 🔧 Miscellaneous

* Add infer agent ([0963b3e](https://github.com/inference-gateway/adk/commit/0963b3eae2324f1e43b1c6d06351024dc049c05d))

## [0.15.3](https://github.com/inference-gateway/adk/compare/v0.15.2...v0.15.3) (2025-11-20)

### 🔧 Miscellaneous

* **deps:** Upgrade Inference Gateway SDK to v1.14.0 ([#116](https://github.com/inference-gateway/adk/issues/116)) ([26722e9](https://github.com/inference-gateway/adk/commit/26722e9ac1cdc5ffaebe10dea3fffae5f75c9f71))

## [0.15.2](https://github.com/inference-gateway/adk/compare/v0.15.1...v0.15.2) (2025-10-18)

### 🐛 Bug Fixes

* **artifacts:** Prevent double initialization of artifacts service ([#115](https://github.com/inference-gateway/adk/issues/115)) ([5f79ef4](https://github.com/inference-gateway/adk/commit/5f79ef4c1ac716f8c129099e7d66a0debe42a75e)), closes [#114](https://github.com/inference-gateway/adk/issues/114)

## [0.15.1](https://github.com/inference-gateway/adk/compare/v0.15.0...v0.15.1) (2025-10-17)

### 🐛 Bug Fixes

* **task:** CancelTask now actually stops running task execution ([#113](https://github.com/inference-gateway/adk/issues/113)) ([9aefbcf](https://github.com/inference-gateway/adk/commit/9aefbcf6b990f987950b8f18d4d161ebce01956b)), closes [#112](https://github.com/inference-gateway/adk/issues/112)

## [0.15.0](https://github.com/inference-gateway/adk/compare/v0.14.0...v0.15.0) (2025-10-14)

### ✨ Features

* **toolbox:** Implement CreateArtifact tool for autonomous artifact creation ([#111](https://github.com/inference-gateway/adk/issues/111)) ([5fe9bf3](https://github.com/inference-gateway/adk/commit/5fe9bf3898fae795f40f7c07a681656ee55245c9)), closes [#109](https://github.com/inference-gateway/adk/issues/109)

## [0.14.0](https://github.com/inference-gateway/adk/compare/v0.13.1...v0.14.0) (2025-10-11)

### ✨ Features

* Add the callbacks ready to be used by the agent ([#94](https://github.com/inference-gateway/adk/issues/94)) ([84632f3](https://github.com/inference-gateway/adk/commit/84632f3d49dad33e4e6271ae5ef2dfe076faad44))
* **types:** Refactor Part deserialization to use concrete types ([#104](https://github.com/inference-gateway/adk/issues/104)) ([22d192e](https://github.com/inference-gateway/adk/commit/22d192e0bc90d7193b2567f11839b2c6a734af7d)), closes [#102](https://github.com/inference-gateway/adk/issues/102)

### ♻️ Improvements

* Consolidate the logic of the agent and remove redundancy ([#93](https://github.com/inference-gateway/adk/issues/93)) ([54e5e71](https://github.com/inference-gateway/adk/commit/54e5e71690596239a673e64a509edd35d4a5f0ad))
* **docker-compose:** Remove port mappings for server service to avoid confusion ([3df7b08](https://github.com/inference-gateway/adk/commit/3df7b083091b4b826d41728015246fbfcf386e32))
* **server:** Remove handler duplication between A2AServerImpl and DefaultA2AProtocolHandler ([#101](https://github.com/inference-gateway/adk/issues/101)) ([75b9c20](https://github.com/inference-gateway/adk/commit/75b9c207ef2fccc76bc8a42ea38e676e41b22509))

### 🐛 Bug Fixes

* Improve validation logic for task handler configuration ([#98](https://github.com/inference-gateway/adk/issues/98)) ([34d6354](https://github.com/inference-gateway/adk/commit/34d6354839d87d1dd63d949f30631d4756cd2e98))

### 👷 CI

* Add Prettier setup and formatting steps for Go and markdown files ([#105](https://github.com/inference-gateway/adk/issues/105)) ([f2a1994](https://github.com/inference-gateway/adk/commit/f2a199403a91d62b0706c88cdc73cfc35ff202de))

### 📚 Documentation

* **examples:** Add input-required flow examples ([#96](https://github.com/inference-gateway/adk/issues/96)) ([17764f3](https://github.com/inference-gateway/adk/commit/17764f36bd2e4b758cf49fbfb9cde2d0985c78c4))

### 🔧 Miscellaneous

* **deps:** Update claude-code to 2.0.8 and install gh cli version 2.81.0 in flox environment ([4d6e41a](https://github.com/inference-gateway/adk/commit/4d6e41a64bf0249d4690d816db6c44ff6d41e981))

## [0.13.1](https://github.com/inference-gateway/adk/compare/v0.13.0...v0.13.1) (2025-10-05)

### ♻️ Improvements

* **artifact:** Improve artifact storage support usage for URI-based artifacts and provide utilities to a2a clients for consistent downloads ([#90](https://github.com/inference-gateway/adk/issues/90)) ([a72ce62](https://github.com/inference-gateway/adk/commit/a72ce6233fd5ea4115edd1042aa7e2279550cb48))

### 🔧 Miscellaneous

* **deps:** Update claude-code version to ^2.0.1 in manifest files ([453cfd9](https://github.com/inference-gateway/adk/commit/453cfd92566c3700ef04fa475d057d52ec1f459f))

## [0.13.0](https://github.com/inference-gateway/adk/compare/v0.12.1...v0.13.0) (2025-10-05)

### ✨ Features

* **server:** Add artifact extraction to default task handlers ([#89](https://github.com/inference-gateway/adk/issues/89)) ([49359ce](https://github.com/inference-gateway/adk/commit/49359ced013db84da303e2f5ed112c032987549e)), closes [#88](https://github.com/inference-gateway/adk/issues/88)

## [0.12.1](https://github.com/inference-gateway/adk/compare/v0.12.0...v0.12.1) (2025-09-30)

### ♻️ Improvements

* Remove confusing setter for artifacts server configuration from A2AServerBuilder and related mocks ([94fc304](https://github.com/inference-gateway/adk/commit/94fc30498826f0576cb826d929b3509e408dec47))
* Simplfy artifacts server and make it more configurable ([#87](https://github.com/inference-gateway/adk/issues/87)) ([9d7c300](https://github.com/inference-gateway/adk/commit/9d7c300ab63c3750d48131a30e20cad424150501))
* Update README examples to reflect enterprise-ready storage configurations and remove outdated comments from tests ([2eb5454](https://github.com/inference-gateway/adk/commit/2eb545455d08464e1c1d931dd2f9379264c86567))

### 📚 Documentation

* **examples:** Add queue storage examples for in-memory and Redis backends ([#85](https://github.com/inference-gateway/adk/issues/85)) ([5be49e0](https://github.com/inference-gateway/adk/commit/5be49e0966695c375f3143edfab27853ad5b70da)), closes [#80](https://github.com/inference-gateway/adk/issues/80)
* **examples:** Add TLS-enabled A2A server example with docker-compose ([#86](https://github.com/inference-gateway/adk/issues/86)) ([05d54ce](https://github.com/inference-gateway/adk/commit/05d54cedb55c5d2cb0377bee9d245b8fd2fb5048)), closes [#81](https://github.com/inference-gateway/adk/issues/81)

## [0.12.0](https://github.com/inference-gateway/adk/compare/v0.11.2...v0.12.0) (2025-09-29)

### ✨ Features

* Implement artifacts server builder with pluggable storage providers ([#73](https://github.com/inference-gateway/adk/issues/73)) ([5253219](https://github.com/inference-gateway/adk/commit/52532192a49b961a8c6eb021bca113307afc0763))

### ♻️ Improvements

* **examples:** Make the examples scenario / feature based ([#75](https://github.com/inference-gateway/adk/issues/75)) ([b52c8fd](https://github.com/inference-gateway/adk/commit/b52c8fd758e9695e13100779a36982bb0855ae42))

### 🐛 Bug Fixes

* **ci:** Bump mockgen version from v0.5.0 to v0.6.0  ([#79](https://github.com/inference-gateway/adk/issues/79)) ([432145a](https://github.com/inference-gateway/adk/commit/432145a5d89b7256e0b7a4dccb6aad10f4e2cc13))

### 📚 Documentation

* Refactor README.md to ensure it's reconciled with the actual recent code changes and reduce noise ([#83](https://github.com/inference-gateway/adk/issues/83)) ([bf9a22b](https://github.com/inference-gateway/adk/commit/bf9a22ba9a59941e1ebfc341e157ac076b32dcdf))
* Update CONTRIBUTING.md to improve clarity and organization of setup instructions and guidelines ([4ccd47d](https://github.com/inference-gateway/adk/commit/4ccd47d84c7b8274e25c056d6659825647f7668f))

### 🔧 Miscellaneous

* Revise CLAUDE.md to improve clarity and organization of development commands and architecture overview ([5ad6eac](https://github.com/inference-gateway/adk/commit/5ad6eaca023a03f8f530d8b139a48eeb49522fab))
* Update agent version from 1.0.0 to 0.1.0 across multiple files ([3c69c77](https://github.com/inference-gateway/adk/commit/3c69c77b46424ec9c5ba54db09f2ee94a26a4703))
* Use the same instructions used by Claude in Copilot ([9a80452](https://github.com/inference-gateway/adk/commit/9a80452bdaafad27d7d293b8636b80cbbc1e6977))

## [0.11.2](https://github.com/inference-gateway/adk/compare/v0.11.1...v0.11.2) (2025-09-26)

### 🐛 Bug Fixes

* **server:** Enrich user messages with Kind and MessageID fields when creating tasks ([#77](https://github.com/inference-gateway/adk/issues/77)) ([9ffb2d9](https://github.com/inference-gateway/adk/commit/9ffb2d9fc3ef80ed069278b9d6b1f67df7d065e8)), closes [#76](https://github.com/inference-gateway/adk/issues/76)

### 🎨 Miscellaneous

* Clean up whitespace in agent and task handler tests for better readability ([b42e020](https://github.com/inference-gateway/adk/commit/b42e020835633e3b043c9b9f793e7ff12887f909))

## [0.11.1](https://github.com/inference-gateway/adk/compare/v0.11.0...v0.11.1) (2025-09-26)

### ♻️ Improvements

* Change the agent card location to align with latest version (0.3.0) of a2a protocol spec ([#74](https://github.com/inference-gateway/adk/issues/74)) ([bf53fbc](https://github.com/inference-gateway/adk/commit/bf53fbcfad5e2440779a26c71b3ff514fdda3f58))

## [0.11.0](https://github.com/inference-gateway/adk/compare/v0.10.1...v0.11.0) (2025-09-17)

### ✨ Features

* **a2a:** Implement proper Context ID history handling ([#70](https://github.com/inference-gateway/adk/issues/70)) ([36490dc](https://github.com/inference-gateway/adk/commit/36490dc26403a183ee669115aebfc67333f9b18a)), closes [#69](https://github.com/inference-gateway/adk/issues/69)

### 🔧 Miscellaneous

* Delete claude Rreview workflow ([a4d38fb](https://github.com/inference-gateway/adk/commit/a4d38fb40a9bcaa650164c58cef5bef041d949a5))

## [0.10.1](https://github.com/inference-gateway/adk/compare/v0.10.0...v0.10.1) (2025-09-15)

### 🐛 Bug Fixes

* **agent:** Fix NewAgentBuilder.WithConfig to preserve user configuration values ([#64](https://github.com/inference-gateway/adk/issues/64)) ([5b0a077](https://github.com/inference-gateway/adk/commit/5b0a0775c1a1b22cfd4c986ef4ea38eb19b37165)), closes [#63](https://github.com/inference-gateway/adk/issues/63)

## [0.10.0](https://github.com/inference-gateway/adk/compare/v0.9.6...v0.10.0) (2025-09-11)

### ✨ Features

* **a2a:** Add artifact support for both server and client ([#60](https://github.com/inference-gateway/adk/issues/60)) ([efe7b2a](https://github.com/inference-gateway/adk/commit/efe7b2a545f882a88cbfb0c47aadd4f3fa81b440)), closes [#59](https://github.com/inference-gateway/adk/issues/59)

### ♻️ Improvements

* **client:** Refactor SendTaskStreaming to be non-blocking ([#62](https://github.com/inference-gateway/adk/issues/62)) ([1f65d2a](https://github.com/inference-gateway/adk/commit/1f65d2a7c5af583b20f97ee67d4875d5266f70c7)), closes [#61](https://github.com/inference-gateway/adk/issues/61)

## [0.9.6](https://github.com/inference-gateway/adk/compare/v0.9.5...v0.9.6) (2025-09-08)

### 🐛 Bug Fixes

* **toolbox:** Improve descriptions for input_required tool and message parameter ([8036645](https://github.com/inference-gateway/adk/commit/8036645e9143c44eca417bc4e1183c7966ecadc2))

## [0.9.5](https://github.com/inference-gateway/adk/compare/v0.9.4...v0.9.5) (2025-09-08)

### 🐛 Bug Fixes

* **streaming:** Handle streaming failure events and update task status ([#57](https://github.com/inference-gateway/adk/issues/57)) ([4b0aca8](https://github.com/inference-gateway/adk/commit/4b0aca893497bf2ca8e43ee0ea4177f948474be1)), closes [#58](https://github.com/inference-gateway/adk/issues/58)

## [0.9.5-rc.8](https://github.com/inference-gateway/adk/compare/v0.9.5-rc.7...v0.9.5-rc.8) (2025-09-08)

### ♻️ Improvements

* **test:** Add toolbox mock generation and implement tool call accumulator tests ([2b40564](https://github.com/inference-gateway/adk/commit/2b405648ea20e9ccab62cdd3b9787bd0a6ac3fab))

## [0.9.5-rc.7](https://github.com/inference-gateway/adk/compare/v0.9.5-rc.6...v0.9.5-rc.7) (2025-09-08)

### 🐛 Bug Fixes

* **agent:** Improve tool call ID generation and improve handling of duplicate function calls ([b286909](https://github.com/inference-gateway/adk/commit/b2869094f679f1c2f5baf156d2b29b9ce98d3c82))

## [0.9.5-rc.6](https://github.com/inference-gateway/adk/compare/v0.9.5-rc.5...v0.9.5-rc.6) (2025-09-08)

### 🐛 Bug Fixes

* **agent:** Improve tool call handling with unique key generation and order tracking ([38baf3f](https://github.com/inference-gateway/adk/commit/38baf3f6bb8f28c4ed4298d11440d7367f7c8ef8))

## [0.9.5-rc.5](https://github.com/inference-gateway/adk/compare/v0.9.5-rc.4...v0.9.5-rc.5) (2025-09-08)

### ♻️ Improvements

* **agent:** Add debug logging for tool call accumulator in RunWithStream ([01d08aa](https://github.com/inference-gateway/adk/commit/01d08aab46c822455a3288f0435752dbe067bcd8))

## [0.9.5-rc.4](https://github.com/inference-gateway/adk/compare/v0.9.5-rc.3...v0.9.5-rc.4) (2025-09-08)

### 🐛 Bug Fixes

* **agent:** Improve handling of tool call arguments and add JSON completeness check ([da66433](https://github.com/inference-gateway/adk/commit/da664334f9501407cedbe91d40700348c89e0ecd))

## [0.9.5-rc.3](https://github.com/inference-gateway/adk/compare/v0.9.5-rc.2...v0.9.5-rc.3) (2025-09-08)

### 🐛 Bug Fixes

* **agent:** Apply the fix also to non-streaming tasks and also update the function ([28c2b23](https://github.com/inference-gateway/adk/commit/28c2b2364c8c78f86d5cabf8ba17c669d0e533b4))

## [0.9.5-rc.2](https://github.com/inference-gateway/adk/compare/v0.9.5-rc.1...v0.9.5-rc.2) (2025-09-08)

### 🐛 Bug Fixes

* **streaming:** Ensure tool calls have unique IDs when missing ([14d55ac](https://github.com/inference-gateway/adk/commit/14d55ac2e7618cf9260a2d18ed5f82961b0e5677))

## [0.9.5-rc.1](https://github.com/inference-gateway/adk/compare/v0.9.4...v0.9.5-rc.1) (2025-09-08)

### 🐛 Bug Fixes

* **streaming:** Handle streaming failure events and update task status ([36c5ca2](https://github.com/inference-gateway/adk/commit/36c5ca2d045105f3c6f0ed5ce9d5923b09afb83c))

## [0.9.4](https://github.com/inference-gateway/adk/compare/v0.9.3...v0.9.4) (2025-09-08)

### 🐛 Bug Fixes

* **server:** Improve the logic of events ([#55](https://github.com/inference-gateway/adk/issues/55)) ([b68d034](https://github.com/inference-gateway/adk/commit/b68d0345892c505897f18522f503df0b25d87252)), closes [#56](https://github.com/inference-gateway/adk/issues/56)

## [0.9.4-rc.15](https://github.com/inference-gateway/adk/compare/v0.9.4-rc.14...v0.9.4-rc.15) (2025-09-08)

### 🐛 Bug Fixes

* **task:** Streamline iteration event handling and remove redundant message logging ([86d9594](https://github.com/inference-gateway/adk/commit/86d95943c7cb1dd4829f23f6df7c7a750500cc26))

## [0.9.4-rc.14](https://github.com/inference-gateway/adk/compare/v0.9.4-rc.13...v0.9.4-rc.14) (2025-09-08)

### 🐛 Bug Fixes

* **task:** Remove redundant handling of tool result messages and failed tool results during interruption ([e333f31](https://github.com/inference-gateway/adk/commit/e333f31af0663c3b430e6c0a49702eef2cde37f8))

## [0.9.4-rc.13](https://github.com/inference-gateway/adk/compare/v0.9.4-rc.12...v0.9.4-rc.13) (2025-09-08)

### 🐛 Bug Fixes

* **task:** Handle interrupted tool calls and log failed results during streaming ([43e495b](https://github.com/inference-gateway/adk/commit/43e495bd4de6aaa87d358fb577a2ce01e77d4275))

## [0.9.4-rc.12](https://github.com/inference-gateway/adk/compare/v0.9.4-rc.11...v0.9.4-rc.12) (2025-09-08)

### 🐛 Bug Fixes

* **task:** Save interrupted task with preserved history and log success or failure ([ef93aa3](https://github.com/inference-gateway/adk/commit/ef93aa34ab552de0bb9d8e47191a55e37fb495fa))

## [0.9.4-rc.11](https://github.com/inference-gateway/adk/compare/v0.9.4-rc.10...v0.9.4-rc.11) (2025-09-08)

### 🐛 Bug Fixes

* **task:** Need to pass the data to preserve during cancellation ([e5c5635](https://github.com/inference-gateway/adk/commit/e5c5635963a46b30ff905af79ef9df42d795caf1))

## [0.9.4-rc.10](https://github.com/inference-gateway/adk/compare/v0.9.4-rc.9...v0.9.4-rc.10) (2025-09-08)

### ♻️ Improvements

* **task:** Ensure messages are stored when the task gets interrupted due to connectivity so it can be resumed later ([84b6ea9](https://github.com/inference-gateway/adk/commit/84b6ea961068a443b3c417fff95c47746488279f))

## [0.9.4-rc.9](https://github.com/inference-gateway/adk/compare/v0.9.4-rc.8...v0.9.4-rc.9) (2025-09-08)

### ♻️ Improvements

* **task:** Add handling for interrupted tasks in streaming events ([1ecf8e7](https://github.com/inference-gateway/adk/commit/1ecf8e73db4e13c1d6ffed5f886aeeca9e65ca2d))

## [0.9.4-rc.8](https://github.com/inference-gateway/adk/compare/v0.9.4-rc.7...v0.9.4-rc.8) (2025-09-08)

### ♻️ Improvements

* **task:** Move task update logic to after successful response write ([551b8c9](https://github.com/inference-gateway/adk/commit/551b8c9f60bf72f0cc11e3f717d2788e0b50e32e))

## [0.9.4-rc.7](https://github.com/inference-gateway/adk/compare/v0.9.4-rc.6...v0.9.4-rc.7) (2025-09-07)

### ♻️ Improvements

* **agent:** Update condition for logging when no tool calls are executed ([29837dd](https://github.com/inference-gateway/adk/commit/29837dd747e9279e11d2555278d5ea43587adc8a))

## [0.9.4-rc.6](https://github.com/inference-gateway/adk/compare/v0.9.4-rc.5...v0.9.4-rc.6) (2025-09-07)

### ♻️ Improvements

* **task:** Add handling for tool result events in task history ([7fb6038](https://github.com/inference-gateway/adk/commit/7fb60389354711b09423f0e79a77e5815391396d))

## [0.9.4-rc.5](https://github.com/inference-gateway/adk/compare/v0.9.4-rc.4...v0.9.4-rc.5) (2025-09-07)

### ♻️ Improvements

* **server:** Improve delta message handling in A2A protocol ([25c9b38](https://github.com/inference-gateway/adk/commit/25c9b38d48b01900c94412bc75132d28eca50478))
* **task:** Remove deprecated HandleTask method and related processing logic ([429cfb0](https://github.com/inference-gateway/adk/commit/429cfb0ec0a6603b847bdb3c6cdde8eead5c1780))

## [0.9.4-rc.4](https://github.com/inference-gateway/adk/compare/v0.9.4-rc.3...v0.9.4-rc.4) (2025-09-07)

### ♻️ Improvements

* **schema:** Download the latest schema ([adbf5af](https://github.com/inference-gateway/adk/commit/adbf5af9c22e130b2291f9f4354870ab43a9c012))
* **tests:** Add 'Kind' field to task initialization in test cases ([eba3526](https://github.com/inference-gateway/adk/commit/eba35265d3ccf9fff13ccd589f9b47dbef574e43))

## [0.9.4-rc.3](https://github.com/inference-gateway/adk/compare/v0.9.4-rc.2...v0.9.4-rc.3) (2025-09-07)

### ♻️ Improvements

* Add A2A protocol handler and fake OpenAI compatible agent implementation ([6c02f09](https://github.com/inference-gateway/adk/commit/6c02f09e79b488008bfff03da3f4da569776cd1d))
* **server:** Update task handler interfaces to support streaming tasks ([1a5ba6b](https://github.com/inference-gateway/adk/commit/1a5ba6b6aad245984807167d47dd65e59bee6577))

## [0.9.4-rc.2](https://github.com/inference-gateway/adk/compare/v0.9.4-rc.1...v0.9.4-rc.2) (2025-09-07)

### ♻️ Improvements

* **server:** Integrate CloudEvents SDK for enhanced streaming message handling and event generation ([0d9e712](https://github.com/inference-gateway/adk/commit/0d9e712011681dbb4c37455ae6b01dec01a0f25a))

## [0.9.4-rc.1](https://github.com/inference-gateway/adk/compare/v0.9.3...v0.9.4-rc.1) (2025-09-07)

### ♻️ Improvements

* **server:** Consolidate the streaming messages and storage capabilities in the agent RunWithStream method ([8935f0a](https://github.com/inference-gateway/adk/commit/8935f0a2982928f6a497c4ec9990848dc1a2a164))
* **tests:** Remove redundant comments in streaming message accumulation tests ([a1f4c42](https://github.com/inference-gateway/adk/commit/a1f4c42afb6a748be17e8bcb5167792d75fff8e9))

### 🐛 Bug Fixes

* **server:** Rename streamMessage to streamDelta for clarity in streaming handler ([b29793a](https://github.com/inference-gateway/adk/commit/b29793a63f6d1a07cde71582e8ff5c9e5f9332d4))
* **server:** Use consolidated message in task status instead of last chunk ([86f5631](https://github.com/inference-gateway/adk/commit/86f56315e0f5f6eaf7baa3c511812ec6717d30d3))

### ✅ Miscellaneous

* **server:** Add comprehensive streaming message accumulation tests ([f3694c3](https://github.com/inference-gateway/adk/commit/f3694c34360ba166bd307ac1b8aada5c611814f3))

## [0.9.3](https://github.com/inference-gateway/adk/compare/v0.9.2...v0.9.3) (2025-09-07)

### 🐛 Bug Fixes

* **streaming:** Accumulate streaming deltas into single assistant message ([#53](https://github.com/inference-gateway/adk/issues/53)) ([541fe8b](https://github.com/inference-gateway/adk/commit/541fe8b78f0fbe22caa3874c9e07276ec5ef52c4)), closes [#52](https://github.com/inference-gateway/adk/issues/52)
* **task:** Allow resuming tasks in any non-completed state ([#51](https://github.com/inference-gateway/adk/issues/51)) ([cb6f2cf](https://github.com/inference-gateway/adk/commit/cb6f2cf815654196ddd0f2a21114460d21adcc8b)), closes [#50](https://github.com/inference-gateway/adk/issues/50)

### 🔨 Miscellaneous

* **env:** Add environment configuration files for flox setup ([567c386](https://github.com/inference-gateway/adk/commit/567c386db28c7f71e7bb72f7067673800dc594e4))

## [0.9.2](https://github.com/inference-gateway/adk/compare/v0.9.1...v0.9.2) (2025-09-02)

### ♻️ Improvements

* **task:** Add sources and generates fields for A2A type generation ([647018c](https://github.com/inference-gateway/adk/commit/647018cd057dd1ee757cfef65afc47d94f3439b4))

### 🐛 Bug Fixes

* **ci:** Update generator tool version to v0.1.2 ([802c809](https://github.com/inference-gateway/adk/commit/802c8096c6777e6d0ddab30eb418a37752532da8))
* **task:** Update generator version to v0.1.2 for A2A type generation ([318f885](https://github.com/inference-gateway/adk/commit/318f885af9037d2e7e72a2f907bc8618ea39c03a))

## [0.9.1](https://github.com/inference-gateway/adk/compare/v0.9.0...v0.9.1) (2025-09-02)

### ♻️ Improvements

* Refactor message part types to use 'any' instead of 'interface{}' ([d4eb538](https://github.com/inference-gateway/adk/commit/d4eb538a3b0d4345656ab3a26c448dfaeeda9bda))

### 🐛 Bug Fixes

* **metadata:** Clear build-time metadata variables and update server builder to use agent card details ([129a020](https://github.com/inference-gateway/adk/commit/129a0209198d95961d0fd5f3d32d852ac6b9c63e))

## [0.9.0](https://github.com/inference-gateway/adk/compare/v0.8.0...v0.9.0) (2025-08-08)

### ✨ Features

* Add default task handlers for polling and streaming scenarios ([#38](https://github.com/inference-gateway/adk/issues/38)) ([5764b22](https://github.com/inference-gateway/adk/commit/5764b22b41c06688a8983154735d11bee3b7852a)), closes [#37](https://github.com/inference-gateway/adk/issues/37)
* **storage:** Implement multi-broker message queue support with Redis ([#45](https://github.com/inference-gateway/adk/issues/45)) ([639daf5](https://github.com/inference-gateway/adk/commit/639daf5e6393ab88b4d6f0e5b3e92ba73e3ecded))

### 📚 Documentation

* Add environment variable configuration examples and Go code snippets to README ([84f59cd](https://github.com/inference-gateway/adk/commit/84f59cdc74d6034245ae1b9eab7e4302f23f4f73))
* Add health check example client for monitoring agent status ([34af129](https://github.com/inference-gateway/adk/commit/34af129c17f30c2c9943780cd2d69b0617685d94))
* Improve configuration section in README with detailed environment variables and examples ([e8af9a9](https://github.com/inference-gateway/adk/commit/e8af9a98948ee95e7f6512135b08194ae1f17bb1))
* Revise examples section in README for clarity and organization ([af152f9](https://github.com/inference-gateway/adk/commit/af152f91850e20940afeae039c38cdb33ae62203))
* Update early stage warning and streamline development setup instructions ([5af66f0](https://github.com/inference-gateway/adk/commit/5af66f0d9077973401b246c71096912dff03c43c))
* Update terminology from "Production Ready" to "Enterprise Ready" in README ([940c18c](https://github.com/inference-gateway/adk/commit/940c18ceb69dc0620a10e1de6c0508d8580d05de))

### 🔧 Miscellaneous

* Add missing task for generating mocks to pre-commit hook ([802987f](https://github.com/inference-gateway/adk/commit/802987fe6fb777aecfeea8875ba2ea006a3ccf47))

## [0.8.0](https://github.com/inference-gateway/adk/compare/v0.7.4...v0.8.0) (2025-08-05)

### ✨ Features

* Implement complete TaskState handling with input-required pausing ([#32](https://github.com/inference-gateway/adk/issues/32)) ([c47130f](https://github.com/inference-gateway/adk/commit/c47130f3bfaa72ba75e6e302ed7ac19741083450)), closes [#31](https://github.com/inference-gateway/adk/issues/31)

### ♻️ Improvements

* **examples:** Update import statements to use 'adk/types' for consistency ([c1f7510](https://github.com/inference-gateway/adk/commit/c1f7510097706d44832b5f4049c6a0ef8d56b755))

### 📚 Documentation

* Improve main documentation and clean up codebase ([#36](https://github.com/inference-gateway/adk/issues/36)) ([1d1ce8e](https://github.com/inference-gateway/adk/commit/1d1ce8eb43cf9f27821d491dbb8fb52f1bd842fd))

### 🔧 Miscellaneous

* Add issue templates for bug reports, feature requests, and refactor requests ([d7307a4](https://github.com/inference-gateway/adk/commit/d7307a402e7e9ac7b8d24390b8b35d894b2dd6dc))
* Download the latest schema from A2A and  generate the go types ([25be465](https://github.com/inference-gateway/adk/commit/25be4650b92219bc1c28cced22c60db5893fad62))

## [0.7.4](https://github.com/inference-gateway/adk/compare/v0.7.3...v0.7.4) (2025-07-29)

### ♻️ Improvements

* Move everything out of adk package - this repo is the adk ([e72cd1e](https://github.com/inference-gateway/adk/commit/e72cd1eb245097f3454667efb79a76d09a4c6b11))
* Update generated types path from 'adk' to 'types' for consistency ([d0617ca](https://github.com/inference-gateway/adk/commit/d0617ca48273c32741846a829255c60453c73a26))
* Update generator package from 'adk' to 'types' for consistency ([bb7619c](https://github.com/inference-gateway/adk/commit/bb7619cb47be58a84653fc97d1e130d0692cec3e))

### 👷 CI

* Update repository name in release workflow from 'a2a' to 'adk' ([4d7d5c9](https://github.com/inference-gateway/adk/commit/4d7d5c9b20b6f14887b6fb743ed06b38ca76fc8f))

### 📚 Documentation

* Add early stage warning to README.md for project clarity ([22a8dbe](https://github.com/inference-gateway/adk/commit/22a8dbed80944ca5441d90a05e0b4af0e1fedb25))
* Update clone instructions and project structure in CONTRIBUTING.md for 'adk' consistency ([51b2544](https://github.com/inference-gateway/adk/commit/51b2544bfd9e98cdb40dfde436a256dc76a2456f))
* Update import paths to remove 'a2a' prefix for consistency ([e81d9ec](https://github.com/inference-gateway/adk/commit/e81d9ec7316f1c8cdaf3b1090397a9e782a8fc3c))
* Update repository links in CONTRIBUTING.md and README.md to reflect new 'adk' naming ([6af543d](https://github.com/inference-gateway/adk/commit/6af543d962de41f5a7c36e1afdaadc46b6fcb317))

### 🔧 Miscellaneous

* Update clean:mocks task to remove 'adk' prefix from mock files path ([628b4f7](https://github.com/inference-gateway/adk/commit/628b4f70f4f02eb547b7de08c9f3aa793ab07b9c))

## [0.7.3](https://github.com/inference-gateway/a2a/compare/v0.7.2...v0.7.3) (2025-07-21)

### 🐛 Bug Fixes

* **config:** Remove default value for AgentURL to enforce explicit configuration ([1547c21](https://github.com/inference-gateway/a2a/commit/1547c21a797aa7b735ef2c821d2867f00b1e229d))

### ✅ Miscellaneous

* **config:** Update AgentURL assertion in LoadWithLookuper test to reflect explicit configuration ([b4c3f4b](https://github.com/inference-gateway/a2a/commit/b4c3f4bbfba61caa998e72e5c09de0c854f43951))

## [0.7.2](https://github.com/inference-gateway/a2a/compare/v0.7.1...v0.7.2) (2025-07-21)

### ♻️ Improvements

* Enhance LoadAgentCardFromFile to support dynamic JSON attribute overrides ([#28](https://github.com/inference-gateway/a2a/issues/28)) ([1873a12](https://github.com/inference-gateway/a2a/commit/1873a12c511a2ce9d7d613080596a23d2b27b9f1))

## [0.7.1](https://github.com/inference-gateway/a2a/compare/v0.7.0...v0.7.1) (2025-07-21)

### ♻️ Improvements

* **AgentCard:** Make it also possible to load Agent Card from raw JSON file - simple approach ([#27](https://github.com/inference-gateway/a2a/issues/27)) ([cadcd8a](https://github.com/inference-gateway/a2a/commit/cadcd8ac535f9aa5415c0eac0cf5ac426aff3800)), closes [#26](https://github.com/inference-gateway/a2a/issues/26)

## [0.7.0](https://github.com/inference-gateway/a2a/compare/v0.6.3...v0.7.0) (2025-07-20)

### ✨ Features

* **telemetry:** Make OTEL prometheus exporter server configurable ([#24](https://github.com/inference-gateway/a2a/issues/24)) ([c6c5c27](https://github.com/inference-gateway/a2a/commit/c6c5c272b41898c7d3e8e81ac8d0138db5afac74))

### ♻️ Improvements

* **config:** Correct placement of AgentURL field in Config struct ([0293591](https://github.com/inference-gateway/a2a/commit/02935919a43480d8f2b3c49a258e4dac8d70ef59))
* **config:** Move port config under SERVER_ prefix ([#20](https://github.com/inference-gateway/a2a/issues/20)) ([2062a5e](https://github.com/inference-gateway/a2a/commit/2062a5e8ef82b9a2134f20bb795790ed6946fe4e))
* **config:** Move TLS config under SERVER_* prefix ([#23](https://github.com/inference-gateway/a2a/issues/23)) ([fe5e3c2](https://github.com/inference-gateway/a2a/commit/fe5e3c221c816e4f191e703289991659e4b3c95a))

## [0.6.3](https://github.com/inference-gateway/a2a/compare/v0.6.2...v0.6.3) (2025-07-20)

### ♻️ Improvements

* **config:** Replace environment variable configurability for agent with LD flags as metadata ([#18](https://github.com/inference-gateway/a2a/issues/18)) ([70c8958](https://github.com/inference-gateway/a2a/commit/70c8958598edfc3367a7eec64f3d3a124f00d444)), closes [#16](https://github.com/inference-gateway/a2a/issues/16)

### 🐛 Bug Fixes

* **docs:** Ensure consistency in the convention ([c971fb3](https://github.com/inference-gateway/a2a/commit/c971fb3afe4bb0deef3786de3e5685fd4a768550))
* **docs:** Update Table of Contents in README to include new sections and improve navigation ([e8ea1c5](https://github.com/inference-gateway/a2a/commit/e8ea1c5b433cc6570744653f155674af4d1c5def))

## [0.6.2](https://github.com/inference-gateway/a2a/compare/v0.6.1...v0.6.2) (2025-07-19)

### 🐛 Bug Fixes

* Add middleware options to skip A2A and MCP in chat completion requests ([eb07011](https://github.com/inference-gateway/a2a/commit/eb070117fe80ea69e9e66d4e1af17605190f980a))

## [0.6.1](https://github.com/inference-gateway/a2a/compare/v0.6.0...v0.6.1) (2025-07-19)

### 🐛 Bug Fixes

* Update sdk dependency to v1.9.0 and add middleware options for chat completion methods ([#15](https://github.com/inference-gateway/a2a/issues/15)) ([ad7a835](https://github.com/inference-gateway/a2a/commit/ad7a835afe1250a565a3a384aabbe6dc104c29e2))

## [0.6.0](https://github.com/inference-gateway/a2a/compare/v0.5.0...v0.6.0) (2025-07-19)

### ✨ Features

* Add configuration to disable health check logs ([#14](https://github.com/inference-gateway/a2a/issues/14)) ([c86dac4](https://github.com/inference-gateway/a2a/commit/c86dac448d26d0d8749f80e90514b5ca4a52f3e7)), closes [#13](https://github.com/inference-gateway/a2a/issues/13)

### 📚 Documentation

* add GetHealth method documentation and examples to README ([#12](https://github.com/inference-gateway/a2a/issues/12)) ([1f5045d](https://github.com/inference-gateway/a2a/commit/1f5045dce3f5ea80c3cb71807f57faed49d2d60c))

## [0.5.0](https://github.com/inference-gateway/a2a/compare/v0.4.0...v0.5.0) (2025-07-18)

### ✨ Features

* Implement GetHealth method for A2A client ([#11](https://github.com/inference-gateway/a2a/issues/11)) ([3e4b8ce](https://github.com/inference-gateway/a2a/commit/3e4b8ce6487cbfd294ef684b846dbb32f931127f))

### 🐛 Bug Fixes

* Correct allowed tools syntax in Claude workflow ([489d1e4](https://github.com/inference-gateway/a2a/commit/489d1e47a227a8ced387d22a6347101002154871))

### 👷 CI

* Add claude GitHub actions 1752789926575 ([#10](https://github.com/inference-gateway/a2a/issues/10)) ([471e03f](https://github.com/inference-gateway/a2a/commit/471e03f2d044ffc6424f45c4228e30235112857c))
* Update branch prefix in Claude workflow configuration ([d67bef3](https://github.com/inference-gateway/a2a/commit/d67bef3eb58768b31c152fee4f50327d92f48ee3))
* Update Claude workflows to install golangci-lint and task ([583a72d](https://github.com/inference-gateway/a2a/commit/583a72db15f3148c2dbbacbef915152aebb31dfa))

### 📚 Documentation

* Add CLAUDE.md for project guidance and development commands ([29b3259](https://github.com/inference-gateway/a2a/commit/29b32590e6d3465461c8ff3b5fadca5bbb6725cf))
* Update contribution guidelines for pushing changes to branches ([bad5c4a](https://github.com/inference-gateway/a2a/commit/bad5c4a0e0729d90ff1e7596ab65a6a109b36bf7))

### 🔨 Miscellaneous

* Add git command to allowed tools in Claude workflows ([fcb145d](https://github.com/inference-gateway/a2a/commit/fcb145d4583fdbef82d099eec1ae5cae4cef6c2d))
* Install Claude code for enhanced functionality ([aca9f74](https://github.com/inference-gateway/a2a/commit/aca9f7457935ed51a5974e6f1dbe030b2c717f8d))

## [0.4.0](https://github.com/inference-gateway/a2a/compare/v0.3.0...v0.4.0) (2025-06-20)

### ✨ Features

* Add message handling and streaming capabilities ([#7](https://github.com/inference-gateway/a2a/issues/7)) ([88a7253](https://github.com/inference-gateway/a2a/commit/88a7253b0025f91d726f2a941bc232bff8440655)), closes [#8](https://github.com/inference-gateway/a2a/issues/8)

## [0.4.0-rc.10](https://github.com/inference-gateway/a2a/compare/v0.4.0-rc.9...v0.4.0-rc.10) (2025-06-20)

### 🐛 Bug Fixes

* Ensure tool_calls comes before tool results in the sequence order ([0067f08](https://github.com/inference-gateway/a2a/commit/0067f08113b0d319443e0deddc7beab65fb803bd))

## [0.4.0-rc.9](https://github.com/inference-gateway/a2a/compare/v0.4.0-rc.8...v0.4.0-rc.9) (2025-06-20)

### 🐛 Bug Fixes

* Improve tool execution handling and status updates in processStream ([1f8dcc2](https://github.com/inference-gateway/a2a/commit/1f8dcc27796b51afcb0ee27b76ac68339d788d25))

## [0.4.0-rc.8](https://github.com/inference-gateway/a2a/compare/v0.4.0-rc.7...v0.4.0-rc.8) (2025-06-20)

### 🐛 Bug Fixes

* Enhance tool execution status updates with tool call ID in response ([cbbc987](https://github.com/inference-gateway/a2a/commit/cbbc98739481a88a1c708dfffe9f5fd41f2b8ac9))

## [0.4.0-rc.7](https://github.com/inference-gateway/a2a/compare/v0.4.0-rc.6...v0.4.0-rc.7) (2025-06-20)

### ♻️ Improvements

* Rename handleLLMStreaming to handleIterativeStreaming and streamline tool execution process ([d065031](https://github.com/inference-gateway/a2a/commit/d0650314e0eaca651b698cf5eebc82c7bc4c38a9))

## [0.4.0-rc.6](https://github.com/inference-gateway/a2a/compare/v0.4.0-rc.5...v0.4.0-rc.6) (2025-06-19)

### 🐛 Bug Fixes

* Update processIterativeStreaming to include tools in CreateStreamingChatCompletion ([102c252](https://github.com/inference-gateway/a2a/commit/102c252d12e6d7daa2c4c37dfef14e7ee4abed15))

## [0.4.0-rc.5](https://github.com/inference-gateway/a2a/compare/v0.4.0-rc.4...v0.4.0-rc.5) (2025-06-19)

### ♻️ Improvements

* Remove unnecessary abstractions and add missing methods to an OpenAICompatibleAgent interface ([a97a073](https://github.com/inference-gateway/a2a/commit/a97a073b85622ac819a3d4e588deada77015b421))

## [0.4.0-rc.4](https://github.com/inference-gateway/a2a/compare/v0.4.0-rc.3...v0.4.0-rc.4) (2025-06-19)

### 🐛 Bug Fixes

* Initialize message handler with agent in NewA2AServer and SetAgent methods ([9ffb752](https://github.com/inference-gateway/a2a/commit/9ffb752163185eced68ca5318f0b7732cb70ab3e))

## [0.4.0-rc.3](https://github.com/inference-gateway/a2a/compare/v0.4.0-rc.2...v0.4.0-rc.3) (2025-06-19)

### 🐛 Bug Fixes

* Update message handling to use SendStreamingMessageResponse for streaming responses ([06b43cb](https://github.com/inference-gateway/a2a/commit/06b43cbcf4630cc7dc281ef5d735831327a09b43))

## [0.4.0-rc.2](https://github.com/inference-gateway/a2a/compare/v0.4.0-rc.1...v0.4.0-rc.2) (2025-06-19)

### 🐛 Bug Fixes

* Update SendTaskStreaming to use Server-Sent Events format for streaming responses ([47b7c28](https://github.com/inference-gateway/a2a/commit/47b7c280bdd6b29ac1a6ce43680151d503bd5bee))

## [0.4.0-rc.1](https://github.com/inference-gateway/a2a/compare/v0.3.0...v0.4.0-rc.1) (2025-06-19)

### ✨ Features

* Add message handling and streaming capabilities ([2e5a2c6](https://github.com/inference-gateway/a2a/commit/2e5a2c6be32e35cf5a59ceb1c59d19c8733a9f89))
* Enhance message handler with timezone support and current timestamp retrieval ([62ec9e9](https://github.com/inference-gateway/a2a/commit/62ec9e9b236d44926c323e6629f9c07fdeb3280e))

## [0.3.0](https://github.com/inference-gateway/a2a/compare/v0.2.0...v0.3.0) (2025-06-18)

### ✨ Features

* Add SetAgentCard and WithAgentCard methods for custom agent card management ([dde3560](https://github.com/inference-gateway/a2a/commit/dde356014bcef1f20761b616944ae27924f932ef))

## [0.2.0](https://github.com/inference-gateway/a2a/compare/v0.1.9...v0.2.0) (2025-06-16)

### ✨ Features

* Add ListTasks functionality to retrieve and paginate tasks ([#5](https://github.com/inference-gateway/a2a/issues/5)) ([1d3ee38](https://github.com/inference-gateway/a2a/commit/1d3ee380fd1c6f31ec73987640f43f460d935d0c))
* Add push notification configuration management for tasks ([#6](https://github.com/inference-gateway/a2a/issues/6)) ([4d3b998](https://github.com/inference-gateway/a2a/commit/4d3b998addb3c4dad1bf8c2196e9a7a33eb83075))

## [0.1.9](https://github.com/inference-gateway/a2a/compare/v0.1.8...v0.1.9) (2025-06-16)

### ♻️ Improvements

* Improve type-safety and overall code structure with fluent interfaces ([#4](https://github.com/inference-gateway/a2a/issues/4)) ([dc53a58](https://github.com/inference-gateway/a2a/commit/dc53a589c00722140aed9fa3f86aa7e8e0cf78ea))

## [0.2.0-rc.11](https://github.com/inference-gateway/a2a/compare/v0.2.0-rc.10...v0.2.0-rc.11) (2025-06-16)

### ♻️ Improvements

* Add MaxConversationHistory to AgentBuilder and corresponding tests ([4f6cce2](https://github.com/inference-gateway/a2a/commit/4f6cce27a7ed9e0837cd8e23036546cc7cb35748))
* Cleanup - removing comments ([d26b280](https://github.com/inference-gateway/a2a/commit/d26b2805cdb5945c3b8e5814e3788417c8476de8))

## [0.2.0-rc.10](https://github.com/inference-gateway/a2a/compare/v0.2.0-rc.9...v0.2.0-rc.10) (2025-06-16)

### ♻️ Improvements

* **example:** Simplify AI agent creation process and enhance configuration handling ([6ad219f](https://github.com/inference-gateway/a2a/commit/6ad219f1a61cd95508f5bbd0982107cc9e5d91cf))
* Handle default struct tags config, if the user passes in a config the other non-set configurations will be populated with default values ([0f2198b](https://github.com/inference-gateway/a2a/commit/0f2198b5fed36825f4310410204531145e4a263b))

## [0.2.0-rc.9](https://github.com/inference-gateway/a2a/compare/v0.2.0-rc.8...v0.2.0-rc.9) (2025-06-16)

### 🐛 Bug Fixes

* Add missing ToolCalls to the conversion from Google's ADK ([6494c50](https://github.com/inference-gateway/a2a/commit/6494c508353f3ccddd355b9d0681af7a4afe8a74))

### ✅ Miscellaneous

* Add tests for tool_calls handling in message conversion ([f5ba845](https://github.com/inference-gateway/a2a/commit/f5ba8458bba46fd46bc9609ff619183be270dfa5))

## [0.2.0-rc.8](https://github.com/inference-gateway/a2a/compare/v0.2.0-rc.7...v0.2.0-rc.8) (2025-06-16)

### ✨ Features

* Add tool_call_id support in message conversion and update tests ([dff354f](https://github.com/inference-gateway/a2a/commit/dff354f4e7484318bab86a3e239b1bef78450720))

## [0.2.0-rc.7](https://github.com/inference-gateway/a2a/compare/v0.2.0-rc.6...v0.2.0-rc.7) (2025-06-16)

### ✨ Features

* Introduce fluent builder interfaces for A2A server and agent configurations ([93b95b8](https://github.com/inference-gateway/a2a/commit/93b95b81d150e95224f1d034dab85a2587f93065))

### ♻️ Improvements

* Enhance TestAgentWithConfig to validate LLM client creation and add default configuration ([42c52fb](https://github.com/inference-gateway/a2a/commit/42c52fbb1c0d981be07cbfc5ab0a01eb65d0ab6e))
* Remove redundant comment in NewWithDefaults function ([b32dd9d](https://github.com/inference-gateway/a2a/commit/b32dd9d3aeacbfd7452c2ceae91618b423c492e9))

## [0.2.0-rc.6](https://github.com/inference-gateway/a2a/compare/v0.2.0-rc.5...v0.2.0-rc.6) (2025-06-16)

### ♻️ Improvements

* Simplify configuration handling and improve defaults application ([da67dee](https://github.com/inference-gateway/a2a/commit/da67deefe9006d52c82442a3f24f055c8054e2bf))

## [0.2.0-rc.5](https://github.com/inference-gateway/a2a/compare/v0.2.0-rc.4...v0.2.0-rc.5) (2025-06-16)

### ♻️ Improvements

* Enhance configuration handling by applying defaults and simplifying server builder initialization ([e255420](https://github.com/inference-gateway/a2a/commit/e25542087620acda55ada43cf0191f90ec770ce7))

## [0.2.0-rc.4](https://github.com/inference-gateway/a2a/compare/v0.2.0-rc.3...v0.2.0-rc.4) (2025-06-15)

### ♻️ Improvements

* Add validation for MaxChatCompletionIterations in Config to be at least 1 ([08bec29](https://github.com/inference-gateway/a2a/commit/08bec2960e0ef04369b919f19f5840eb6bbc44c1))

## [0.2.0-rc.3](https://github.com/inference-gateway/a2a/compare/v0.2.0-rc.2...v0.2.0-rc.3) (2025-06-15)

### ♻️ Improvements

* Enhance AI-Powered A2A Server example with improved configuration handling and error messages ([9a70637](https://github.com/inference-gateway/a2a/commit/9a706370c7a23a0530b735d1645b74d7bf1e732c))
* Extract the logic of message conversions into a dedicated util ([f461c7f](https://github.com/inference-gateway/a2a/commit/f461c7f5810ba0903b460c7c8be15e55f3b553f6))
* Remove Community section from README ([fd151fa](https://github.com/inference-gateway/a2a/commit/fd151fa5259a53d9a1b93a0b1386ecdf3744cdf1))
* Remove Examples Repository link from README ([6a42c64](https://github.com/inference-gateway/a2a/commit/6a42c6476fe567ac54a0ff90a80283f981bf31b2))
* Remove HealthCheck method and related stubs from LLMClient and FakeLLMClient ([d15538f](https://github.com/inference-gateway/a2a/commit/d15538f4e7f8d1c890b4240ee8a2287ebf685abb))
* Remove unused A2AConversationManager and related types - I moved all to a utils package ([654bff8](https://github.com/inference-gateway/a2a/commit/654bff8abf60479ed6c6c0448db673e15b5ed952))
* Simplify message part validation logic in ConvertFromSDK test ([928bd3c](https://github.com/inference-gateway/a2a/commit/928bd3cebe5ba0b26e622fdbef602a51d5a22a80))
* Update README for A2A Server examples with clearer descriptions and improved structure ([ac8490c](https://github.com/inference-gateway/a2a/commit/ac8490c154bf2aff201d61d49c2b1d62f5487b26))

### 🐛 Bug Fixes

* Implement SimpleTaskHandler for basic task processing without AI and add kind Task to the tasks/get object instead of empty string ([dbc413a](https://github.com/inference-gateway/a2a/commit/dbc413a35489fde602fc7413a5c8079c3f265f9c))

### 📚 Documentation

* Remove community section from README ([bf6fd0b](https://github.com/inference-gateway/a2a/commit/bf6fd0b2a89a774214ee160329eb0f0d7d7db2b2))
* Update date and add A2A official documentation link ([3991b6a](https://github.com/inference-gateway/a2a/commit/3991b6a666aacd823e4a7fb01f2cddbc38ed6e7a))

### 🔧 Miscellaneous

* Add MessagePartKind type and validation for A2A message parts ([fccf391](https://github.com/inference-gateway/a2a/commit/fccf3914e06dcded96158e03d3b07c0dfbb4ec31))

## [0.2.0-rc.2](https://github.com/inference-gateway/a2a/compare/v0.2.0-rc.1...v0.2.0-rc.2) (2025-06-15)

### ♻️ Improvements

* Enhance message conversion logging in convertToSDKMessages ([e6b0bd8](https://github.com/inference-gateway/a2a/commit/e6b0bd8c4456d5f6aa5af3e4761ec8ec9a8471e0))

## [0.2.0-rc.1](https://github.com/inference-gateway/a2a/compare/v0.1.9-rc.5...v0.2.0-rc.1) (2025-06-15)

### ✨ Features

* Implement conversation history management in TaskManager ([ce299e5](https://github.com/inference-gateway/a2a/commit/ce299e531a87bfcfdea1ef3cb459371a040e791c))

## [0.1.9-rc.5](https://github.com/inference-gateway/a2a/compare/v0.1.9-rc.4...v0.1.9-rc.5) (2025-06-15)

### ♻️ Improvements

* Add context_id to logging for better traceability in task processing ([fbe1e4e](https://github.com/inference-gateway/a2a/commit/fbe1e4e6bcb9f18504fd32aa473c90c19e241098))

## [0.1.9-rc.4](https://github.com/inference-gateway/a2a/compare/v0.1.9-rc.3...v0.1.9-rc.4) (2025-06-15)

### 🐛 Bug Fixes

* Update SendTask and SendTaskStreaming to use getA2AEndpointURL for constructing the endpoint URL ([fa58951](https://github.com/inference-gateway/a2a/commit/fa589512bf89f7b55c0fa6c2ce6322ecd17dd06c))

## [0.1.9-rc.3](https://github.com/inference-gateway/a2a/compare/v0.1.9-rc.2...v0.1.9-rc.3) (2025-06-15)

### 🐛 Bug Fixes

* Have to ensure the assistant message is first appended with the tool_calls before I can add a message of role tool with the results ([1b679f5](https://github.com/inference-gateway/a2a/commit/1b679f5309db415f530d71101fca27a49d50b005))

## [0.1.9-rc.2](https://github.com/inference-gateway/a2a/compare/v0.1.9-rc.1...v0.1.9-rc.2) (2025-06-15)

### 🐛 Bug Fixes

* Add toolCallId handling in convertToSDKMessages for messages with the role tool ([87b4d64](https://github.com/inference-gateway/a2a/commit/87b4d64840a21b5a973d6bfcba2cdea24d521307))

## [0.1.9-rc.1](https://github.com/inference-gateway/a2a/compare/v0.1.8...v0.1.9-rc.1) (2025-06-15)

### 🐛 Bug Fixes

* Update tool result message structure and enhance test coverage for tool processing ([2937e0f](https://github.com/inference-gateway/a2a/commit/2937e0fbdb114ba80eb6406b607525452befbb36))

## [0.1.8](https://github.com/inference-gateway/a2a/compare/v0.1.7...v0.1.8) (2025-06-15)

### 🐛 Bug Fixes

* Empty message for tool processing - when pulling a task from the Queue we need to check the task.Status.Message which is where the task handler is storing the original message ([703dfba](https://github.com/inference-gateway/a2a/commit/703dfba8cf033a33385da66692bc70e49b61d531))

## [0.1.7](https://github.com/inference-gateway/a2a/compare/v0.1.6...v0.1.7) (2025-06-15)

### 🐛 Bug Fixes

* Update API key validation to be optional in NewOpenAICompatibleLLMClient ([ea0b1bd](https://github.com/inference-gateway/a2a/commit/ea0b1bd73111266da8c241ad51d38c7b82a2d5bd))

### 📚 Documentation

* Update CONTRIBUTING.md for clarity on type generation and mocks ([438fb91](https://github.com/inference-gateway/a2a/commit/438fb91254e0db9e5e8d8198ec619c0e4fa4c2cd))

## [0.1.6](https://github.com/inference-gateway/a2a/compare/v0.1.5...v0.1.6) (2025-06-14)

### 📚 Documentation

* **fix:** Update README title and improve section organization ([03a086e](https://github.com/inference-gateway/a2a/commit/03a086ea0154c1c097c381c95eca34eb7156a7c5))
* Remove "Advanced Authentication" item from the roadmap ([322579e](https://github.com/inference-gateway/a2a/commit/322579e7247e377754e105efb17a2df948330a67))
* Remove redundant Table of Contents section from README.md ([4c9e33a](https://github.com/inference-gateway/a2a/commit/4c9e33a7ed29926f26b404b6b66635252ba619c7))
* Update README.md with enhanced table of contents and usage examples ([00d31eb](https://github.com/inference-gateway/a2a/commit/00d31eb78060d2b8cf4c0eddbc7a9a46bf4654a7))

### 🔧 Miscellaneous

* Update README.md ([e3a9480](https://github.com/inference-gateway/a2a/commit/e3a9480ad5cec677b1d494456c7cfb16a9efbe15))

## [0.1.5](https://github.com/inference-gateway/a2a/compare/v0.1.4...v0.1.5) (2025-06-14)

### ✅ Miscellaneous

* Test the configurations and ensure everything works as expected ([#3](https://github.com/inference-gateway/a2a/issues/3)) ([aa0629e](https://github.com/inference-gateway/a2a/commit/aa0629e74a890c516263b343c5e5c20acae13523))

## [0.1.4](https://github.com/inference-gateway/a2a/compare/v0.1.3...v0.1.4) (2025-06-14)

### ♻️ Improvements

* Cleanup - remove unintended usage of functions ([#2](https://github.com/inference-gateway/a2a/issues/2)) ([9275cb5](https://github.com/inference-gateway/a2a/commit/9275cb5208ccdf75503e2d8939e0bdd784cd7d25))

## [0.1.3](https://github.com/inference-gateway/a2a/compare/v0.1.2...v0.1.3) (2025-06-14)

### ♻️ Improvements

* Improve the structure of the library ([#1](https://github.com/inference-gateway/a2a/issues/1)) ([b41c040](https://github.com/inference-gateway/a2a/commit/b41c0400710eae83a55e07b41067d5b0966a0fe9))

## [0.1.2](https://github.com/inference-gateway/a2a/compare/v0.1.1...v0.1.2) (2025-06-12)

### ♻️ Improvements

* Replace a2a types with local types in agent.go and update go.mod/go.sum ([35274b6](https://github.com/inference-gateway/a2a/commit/35274b6bb31f1ae1b42fb122285a6d0ba45dcd09))

### 🐛 Bug Fixes

* Update documentation link for Core Inference Gateway ([0fd71ce](https://github.com/inference-gateway/a2a/commit/0fd71cef658ddafe45206072762f0b9d48e15ce5))

## [0.1.1](https://github.com/inference-gateway/a2a/compare/v0.1.0...v0.1.1) (2025-06-12)

### ♻️ Improvements

* Add clean task to remove build artifacts ([b2f0030](https://github.com/inference-gateway/a2a/commit/b2f00307dfc5c87f4cfc110b4d7b2df5318a84b2))
* Remove unused semantic release plugin from Dockerfile and release workflow ([66febcf](https://github.com/inference-gateway/a2a/commit/66febcfb6b77d7af412f45e241be1b96d9fb1283))

### 🐛 Bug Fixes

* Update repository URL in release configuration ([bc4606b](https://github.com/inference-gateway/a2a/commit/bc4606b74a3f93954070abb551b960d452c247b4))
