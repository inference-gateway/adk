# Changelog

All notable changes to this project will be documented in this file.

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
