// Code generated from JSON schema. DO NOT EDIT.
package types

import "time"

// --8<-- [start:APIKeySecurityScheme] Defines a security scheme using an API key.
type APIKeySecurityScheme struct {
	Description *string `json:"description,omitempty"`
	Location *string `json:"location,omitempty"`
	Name *string `json:"name,omitempty"`
}

// --8<-- [start:AgentCapabilities] Defines optional capabilities supported by an agent.
type AgentCapabilities struct {
	Extensions []AgentExtension `json:"extensions,omitempty"`
	PushNotifications *bool `json:"push_notifications,omitempty"`
	StateTransitionHistory *bool `json:"state_transition_history,omitempty"`
	Streaming *bool `json:"streaming,omitempty"`
}

// --8<-- [start:AgentCard] AgentCard is a self-describing manifest for an agent. It provides essential metadata including the agent's identity, capabilities, skills, supported communication methods, and security requirements. Next ID: 20
type AgentCard struct {
	AdditionalInterfaces []AgentInterface `json:"additional_interfaces,omitempty"`
	Capabilities *AgentCapabilities `json:"capabilities,omitempty"`
	DefaultInputModes []string `json:"default_input_modes,omitempty"`
	DefaultOutputModes []string `json:"default_output_modes,omitempty"`
	Description *string `json:"description,omitempty"`
	DocumentationURL *string `json:"documentation_url,omitempty"`
	IconURL *string `json:"icon_url,omitempty"`
	Name *string `json:"name,omitempty"`
	PreferredTransport *string `json:"preferred_transport,omitempty"`
	ProtocolVersion *string `json:"protocol_version,omitempty"`
	Provider *AgentProvider `json:"provider,omitempty"`
	Security []Security `json:"security,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"security_schemes,omitempty"`
	Signatures []AgentCardSignature `json:"signatures,omitempty"`
	Skills []AgentSkill `json:"skills,omitempty"`
	SupportedInterfaces []AgentInterface `json:"supported_interfaces,omitempty"`
	SupportsAuthenticatedExtendedCard *bool `json:"supports_authenticated_extended_card,omitempty"`
	URL *string `json:"url,omitempty"`
	Version *string `json:"version,omitempty"`
}

// --8<-- [start:AgentCardSignature] AgentCardSignature represents a JWS signature of an AgentCard. This follows the JSON format of an RFC 7515 JSON Web Signature (JWS).
type AgentCardSignature struct {
	Header map[string]any `json:"header,omitempty"`
	Protected *string `json:"protected,omitempty"`
	Signature *string `json:"signature,omitempty"`
}

// --8<-- [start:AgentExtension] A declaration of a protocol extension supported by an Agent.
type AgentExtension struct {
	Description *string `json:"description,omitempty"`
	Params map[string]any `json:"params,omitempty"`
	Required *bool `json:"required,omitempty"`
	URI *string `json:"uri,omitempty"`
}

// --8<-- [start:AgentInterface] Declares a combination of a target URL and a transport protocol for interacting with the agent. This allows agents to expose the same functionality over multiple protocol binding mechanisms.
type AgentInterface struct {
	ProtocolBinding *string `json:"protocol_binding,omitempty"`
	URL *string `json:"url,omitempty"`
}

// --8<-- [start:AgentProvider] Represents the service provider of an agent.
type AgentProvider struct {
	Organization *string `json:"organization,omitempty"`
	URL *string `json:"url,omitempty"`
}

// --8<-- [start:AgentSkill] Represents a distinct capability or function that an agent can perform.
type AgentSkill struct {
	Description *string `json:"description,omitempty"`
	Examples []string `json:"examples,omitempty"`
	ID *string `json:"id,omitempty"`
	InputModes []string `json:"input_modes,omitempty"`
	Name *string `json:"name,omitempty"`
	OutputModes []string `json:"output_modes,omitempty"`
	Security []Security `json:"security,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

// --8<-- [start:Artifact] Artifacts represent task outputs.
type Artifact struct {
	ArtifactID *string `json:"artifact_id,omitempty"`
	Description *string `json:"description,omitempty"`
	Extensions []string `json:"extensions,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Name *string `json:"name,omitempty"`
	Parts []Part `json:"parts,omitempty"`
}

// --8<-- [start:PushNotificationAuthenticationInfo] Defines authentication details, used for push notifications.
type AuthenticationInfo struct {
	Credentials *string `json:"credentials,omitempty"`
	Schemes []string `json:"schemes,omitempty"`
}

// --8<-- [start:AuthorizationCodeOAuthFlow] Defines configuration details for the OAuth 2.0 Authorization Code flow.
type AuthorizationCodeOAuthFlow struct {
	AuthorizationURL *string `json:"authorization_url,omitempty"`
	RefreshURL *string `json:"refresh_url,omitempty"`
	Scopes map[string]string `json:"scopes,omitempty"`
	TokenURL *string `json:"token_url,omitempty"`
}

// --8<-- [start:CancelTaskRequest] Represents a request for the `tasks/cancel` method.
type CancelTaskRequest struct {
	Name *string `json:"name,omitempty"`
}

// --8<-- [start:ClientCredentialsOAuthFlow] Defines configuration details for the OAuth 2.0 Client Credentials flow.
type ClientCredentialsOAuthFlow struct {
	RefreshURL *string `json:"refresh_url,omitempty"`
	Scopes map[string]string `json:"scopes,omitempty"`
	TokenURL *string `json:"token_url,omitempty"`
}

// --8<-- [start:DataPart] DataPart represents a structured blob.
type DataPart struct {
	Data map[string]any `json:"data,omitempty"`
}

// --8<-- [start:DeleteTaskPushNotificationConfigRequest] Represents a request for the `tasks/pushNotificationConfig/delete` method.
type DeleteTaskPushNotificationConfigRequest struct {
	Name *string `json:"name,omitempty"`
}

// --8<-- [start:FilePart] FilePart represents the different ways files can be provided. If files are small, directly feeding the bytes is supported via file_with_bytes. If the file is large, the agent should read the content as appropriate directly from the file_with_uri source.
type FilePart any

// --8<-- [start:GetExtendedAgentCardRequest]  Empty. Added to fix linter violation.
type GetExtendedAgentCardRequest struct {
}

// --8<-- [start:GetTaskPushNotificationConfigRequest]
type GetTaskPushNotificationConfigRequest struct {
	Name *string `json:"name,omitempty"`
}

// --8<-- [start:GetTaskRequest] Represents a request for the `tasks/get` method.
type GetTaskRequest struct {
	HistoryLength *int `json:"history_length,omitempty"`
	Name *string `json:"name,omitempty"`
}

// --8<-- [start:HTTPAuthSecurityScheme] Defines a security scheme using HTTP authentication.
type HTTPAuthSecurityScheme struct {
	BearerFormat *string `json:"bearer_format,omitempty"`
	Description *string `json:"description,omitempty"`
	Scheme *string `json:"scheme,omitempty"`
}

// --8<-- [start:ImplicitOAuthFlow] Defines configuration details for the OAuth 2.0 Implicit flow.
type ImplicitOAuthFlow struct {
	AuthorizationURL *string `json:"authorization_url,omitempty"`
	RefreshURL *string `json:"refresh_url,omitempty"`
	Scopes map[string]string `json:"scopes,omitempty"`
}

// --8<-- [start:ListTaskPushNotificationConfigRequest]
type ListTaskPushNotificationConfigRequest struct {
	PageSize *int `json:"page_size,omitempty"`
	PageToken *string `json:"page_token,omitempty"`
	Parent *string `json:"parent,omitempty"`
}

// --8<-- [start:ListTaskPushNotificationConfigResponse] Represents a successful response for the `tasks/pushNotificationConfig/list` method.
type ListTaskPushNotificationConfigResponse struct {
	Configs []TaskPushNotificationConfig `json:"configs,omitempty"`
	NextPageToken *string `json:"next_page_token,omitempty"`
}

// --8<-- [start:ListTasksRequest] Parameters for listing tasks with optional filtering criteria.
type ListTasksRequest struct {
	ContextID *string `json:"context_id,omitempty"`
	HistoryLength *int `json:"history_length,omitempty"`
	IncludeArtifacts *bool `json:"include_artifacts,omitempty"`
	LastUpdatedAfter *string `json:"last_updated_after,omitempty"`
	PageSize *int `json:"page_size,omitempty"`
	PageToken *string `json:"page_token,omitempty"`
	Status *any `json:"status,omitempty"`
}

// --8<-- [start:ListTasksResponse] Result object for tasks/list method containing an array of tasks and pagination information.
type ListTasksResponse struct {
	NextPageToken *string `json:"next_page_token,omitempty"`
	PageSize *int `json:"page_size,omitempty"`
	Tasks []Task `json:"tasks,omitempty"`
	TotalSize *int `json:"total_size,omitempty"`
}

// --8<-- [start:Message] Message is one unit of communication between client and server. It is associated with a context and optionally a task. Since the server is responsible for the context definition, it must always provide a context_id in its messages. The client can optionally provide the context_id if it knows the context to associate the message to. Similarly for task_id, except the server decides if a task is created and whether to include the task_id.
type Message struct {
	ContextID *string `json:"context_id,omitempty"`
	Extensions []string `json:"extensions,omitempty"`
	MessageID *string `json:"message_id,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Parts []Part `json:"parts,omitempty"`
	ReferenceTaskIds []string `json:"reference_task_ids,omitempty"`
	Role *any `json:"role,omitempty"`
	TaskID *string `json:"task_id,omitempty"`
}

// --8<-- [start:MutualTLSSecurityScheme] Defines a security scheme using mTLS authentication.
type MutualTlsSecurityScheme struct {
	Description *string `json:"description,omitempty"`
}

// --8<-- [start:OAuth2SecurityScheme] Defines a security scheme using OAuth 2.0.
type OAuth2SecurityScheme struct {
	Description *string `json:"description,omitempty"`
	Flows *OAuthFlows `json:"flows,omitempty"`
	Oauth2MetadataURL *string `json:"oauth2_metadata_url,omitempty"`
}

// --8<-- [start:OAuthFlows] Defines the configuration for the supported OAuth 2.0 flows.
type OAuthFlows any

// --8<-- [start:OpenIdConnectSecurityScheme] Defines a security scheme using OpenID Connect.
type OpenIdConnectSecurityScheme struct {
	Description *string `json:"description,omitempty"`
	OpenIDConnectURL *string `json:"open_id_connect_url,omitempty"`
}

// --8<-- [start:Part] Part represents a container for a section of communication content. Parts can be purely textual, some sort of file (image, video, etc) or a structured data blob (i.e. JSON).
type Part any

// --8<-- [start:PasswordOAuthFlow] Defines configuration details for the OAuth 2.0 Resource Owner Password flow.
type PasswordOAuthFlow struct {
	RefreshURL *string `json:"refresh_url,omitempty"`
	Scopes map[string]string `json:"scopes,omitempty"`
	TokenURL *string `json:"token_url,omitempty"`
}

// --8<-- [start:PushNotificationConfig] Configuration for setting up push notifications for task updates.
type PushNotificationConfig struct {
	Authentication *AuthenticationInfo `json:"authentication,omitempty"`
	ID *string `json:"id,omitempty"`
	Token *string `json:"token,omitempty"`
	URL *string `json:"url,omitempty"`
}

type Security struct {
	Schemes map[string]StringList `json:"schemes,omitempty"`
}

// --8<-- [start:SecurityScheme] Defines a security scheme that can be used to secure an agent's endpoints. This is a discriminated union type based on the OpenAPI 3.2 Security Scheme Object. See: https://spec.openapis.org/oas/v3.2.0.html#security-scheme-object
type SecurityScheme any

// /////// Data Model ////////////  --8<-- [start:SendMessageConfiguration] Configuration of a send message request.
type SendMessageConfiguration struct {
	AcceptedOutputModes []string `json:"accepted_output_modes,omitempty"`
	Blocking *bool `json:"blocking,omitempty"`
	HistoryLength *int `json:"history_length,omitempty"`
	PushNotificationConfig *PushNotificationConfig `json:"push_notification_config,omitempty"`
}

// /////////// Request Messages /////////// --8<-- [start:SendMessageRequest] Represents a request for the `message/send` method.
type SendMessageRequest struct {
	Configuration *SendMessageConfiguration `json:"configuration,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Request *Message `json:"request,omitempty"`
}

// ////// Response Messages /////////// --8<-- [start:SendMessageResponse]
type SendMessageResponse any

// --8<-- [start:SetTaskPushNotificationConfigRequest] Represents a request for the `tasks/pushNotificationConfig/set` method.
type SetTaskPushNotificationConfigRequest struct {
	Config *TaskPushNotificationConfig `json:"config,omitempty"`
	ConfigID *string `json:"config_id,omitempty"`
	Parent *string `json:"parent,omitempty"`
}

// --8<-- [start:StreamResponse] A wrapper object used in streaming operations to encapsulate different types of response data.
type StreamResponse any

// protolint:disable REPEATED_FIELD_NAMES_PLURALIZED
type StringList struct {
	List []string `json:"list,omitempty"`
}

// --8<-- [start:SubscribeToTaskRequest]
type SubscribeToTaskRequest struct {
	Name *string `json:"name,omitempty"`
}

// --8<-- [start:Task] Task is the core unit of action for A2A. It has a current status and when results are created for the task they are stored in the artifact. If there are multiple turns for a task, these are stored in history.
type Task struct {
	Artifacts []Artifact `json:"artifacts,omitempty"`
	ContextID *string `json:"context_id,omitempty"`
	History []Message `json:"history,omitempty"`
	ID *string `json:"id,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Status *TaskStatus `json:"status,omitempty"`
}

// --8<-- [start:TaskArtifactUpdateEvent] TaskArtifactUpdateEvent represents a task delta where an artifact has been generated.
type TaskArtifactUpdateEvent struct {
	Append *bool `json:"append,omitempty"`
	Artifact *Artifact `json:"artifact,omitempty"`
	ContextID *string `json:"context_id,omitempty"`
	LastChunk *bool `json:"last_chunk,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	TaskID *string `json:"task_id,omitempty"`
}

// --8<-- [start:TaskPushNotificationConfig] A container associating a push notification configuration with a specific task.
type TaskPushNotificationConfig struct {
	Name *string `json:"name,omitempty"`
	PushNotificationConfig *PushNotificationConfig `json:"push_notification_config,omitempty"`
}

// --8<-- [start:TaskStatus] A container for the status of a task
type TaskStatus struct {
	Message *Message `json:"message,omitempty"`
	State *any `json:"state,omitempty"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
}

// --8<-- [start:TaskStatusUpdateEvent] An event sent by the agent to notify the client of a change in a task's status.
type TaskStatusUpdateEvent struct {
	ContextID *string `json:"context_id,omitempty"`
	Final *bool `json:"final,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Status *TaskStatus `json:"status,omitempty"`
	TaskID *string `json:"task_id,omitempty"`
}

// --8<-- [start:APIKeySecurityScheme] Defines a security scheme using an API key.
