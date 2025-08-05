# Changelog

All notable changes to this project will be documented in this file.

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
