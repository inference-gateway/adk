// Code generated from JSON schema. DO NOT EDIT.
package types

import "time"

// Identifies the sender of the message.
type Role string

// Role enum values
const (
	RoleAgent       Role = "ROLE_AGENT"
	RoleUnspecified Role = "ROLE_UNSPECIFIED"
	RoleUser        Role = "ROLE_USER"
)

// Filter tasks by their current status state.
type TaskState string

// TaskState enum values
const (
	TaskStateAuthRequired  TaskState = "TASK_STATE_AUTH_REQUIRED"
	TaskStateCancelled     TaskState = "TASK_STATE_CANCELLED"
	TaskStateCompleted     TaskState = "TASK_STATE_COMPLETED"
	TaskStateFailed        TaskState = "TASK_STATE_FAILED"
	TaskStateInputRequired TaskState = "TASK_STATE_INPUT_REQUIRED"
	TaskStateRejected      TaskState = "TASK_STATE_REJECTED"
	TaskStateSubmitted     TaskState = "TASK_STATE_SUBMITTED"
	TaskStateUnspecified   TaskState = "TASK_STATE_UNSPECIFIED"
	TaskStateWorking       TaskState = "TASK_STATE_WORKING"
)

// Defines a security scheme using an API key.
type APIKeySecurityScheme struct {
	Description *string `json:"description,omitempty"`
	Location    string  `json:"location"`
	Name        string  `json:"name"`
}

// Defines optional capabilities supported by an agent.
type AgentCapabilities struct {
	Extensions             []AgentExtension `json:"extensions,omitempty"`
	PushNotifications      *bool            `json:"pushNotifications,omitempty"`
	StateTransitionHistory *bool            `json:"stateTransitionHistory,omitempty"`
	Streaming              *bool            `json:"streaming,omitempty"`
}

// AgentCard is a self-describing manifest for an agent. It provides essential
// metadata including the agent's identity, capabilities, skills, supported
// communication methods, and security requirements.
// Next ID: 20
type AgentCard struct {
	AdditionalInterfaces      []AgentInterface          `json:"additionalInterfaces,omitempty"`
	Capabilities              AgentCapabilities         `json:"capabilities"`
	DefaultInputModes         []string                  `json:"defaultInputModes"`
	DefaultOutputModes        []string                  `json:"defaultOutputModes"`
	Description               string                    `json:"description"`
	DocumentationURL          *string                   `json:"documentationUrl,omitempty"`
	IconURL                   *string                   `json:"iconUrl,omitempty"`
	Name                      string                    `json:"name"`
	PreferredTransport        *string                   `json:"preferredTransport,omitempty"`
	ProtocolVersion           string                    `json:"protocolVersion"`
	Provider                  *AgentProvider            `json:"provider,omitempty"`
	Security                  []Security                `json:"security,omitempty"`
	SecuritySchemes           map[string]SecurityScheme `json:"securitySchemes,omitempty"`
	Signatures                []AgentCardSignature      `json:"signatures,omitempty"`
	Skills                    []AgentSkill              `json:"skills"`
	SupportedInterfaces       []AgentInterface          `json:"supportedInterfaces,omitempty"`
	SupportsExtendedAgentCard *bool                     `json:"supportsExtendedAgentCard,omitempty"`
	URL                       *string                   `json:"url,omitempty"`
	Version                   string                    `json:"version"`
}

// AgentCardSignature represents a JWS signature of an AgentCard.
// This follows the JSON format of an RFC 7515 JSON Web Signature (JWS).
type AgentCardSignature struct {
	Header    *Struct `json:"header,omitempty"`
	Protected string  `json:"protected"`
	Signature string  `json:"signature"`
}

// A declaration of a protocol extension supported by an Agent.
type AgentExtension struct {
	Description string  `json:"description"`
	Params      *Struct `json:"params,omitempty"`
	Required    bool    `json:"required"`
	URI         string  `json:"uri"`
}

// Declares a combination of a target URL and a transport protocol for interacting with the agent.
// This allows agents to expose the same functionality over multiple protocol binding mechanisms.
type AgentInterface struct {
	ProtocolBinding string  `json:"protocolBinding"`
	Tenant          *string `json:"tenant,omitempty"`
	URL             string  `json:"url"`
}

// Represents the service provider of an agent.
type AgentProvider struct {
	Organization string `json:"organization"`
	URL          string `json:"url"`
}

// Represents a distinct capability or function that an agent can perform.
type AgentSkill struct {
	Description string     `json:"description"`
	Examples    []string   `json:"examples,omitempty"`
	ID          string     `json:"id"`
	InputModes  []string   `json:"inputModes,omitempty"`
	Name        string     `json:"name"`
	OutputModes []string   `json:"outputModes,omitempty"`
	Security    []Security `json:"security,omitempty"`
	Tags        []string   `json:"tags"`
}

// Artifacts represent task outputs.
type Artifact struct {
	ArtifactID  string   `json:"artifactId"`
	Description *string  `json:"description,omitempty"`
	Extensions  []string `json:"extensions,omitempty"`
	Metadata    *Struct  `json:"metadata,omitempty"`
	Name        *string  `json:"name,omitempty"`
	Parts       []Part   `json:"parts"`
}

// Defines authentication details, used for push notifications.
type AuthenticationInfo struct {
	Credentials *string  `json:"credentials,omitempty"`
	Schemes     []string `json:"schemes"`
}

// Defines configuration details for the OAuth 2.0 Authorization Code flow.
type AuthorizationCodeOAuthFlow struct {
	AuthorizationURL string            `json:"authorizationUrl"`
	RefreshURL       *string           `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
	TokenURL         string            `json:"tokenUrl"`
}

// Represents a request for the `tasks/cancel` method.
type CancelTaskRequest struct {
	Name   string `json:"name"`
	Tenant string `json:"tenant"`
}

// Defines configuration details for the OAuth 2.0 Client Credentials flow.
type ClientCredentialsOAuthFlow struct {
	RefreshURL *string           `json:"refreshUrl,omitempty"`
	Scopes     map[string]string `json:"scopes"`
	TokenURL   string            `json:"tokenUrl"`
}

// DataPart represents a structured blob.
type DataPart struct {
	Data Struct `json:"data"`
}

// Represents a request for the `tasks/pushNotificationConfig/delete` method.
type DeleteTaskPushNotificationConfigRequest struct {
	Name   string `json:"name"`
	Tenant string `json:"tenant"`
}

// FilePart represents the different ways files can be provided. If files are
// small, directly feeding the bytes is supported via file_with_bytes. If the
// file is large, the agent should read the content as appropriate directly
// from the file_with_uri source.
type FilePart struct {
	FileWithBytes *string `json:"fileWithBytes,omitempty"`
	FileWithURI   *string `json:"fileWithUri,omitempty"`
	MediaType     string  `json:"mediaType"`
	Name          string  `json:"name"`
}

type GetExtendedAgentCardRequest struct {
	Tenant string `json:"tenant"`
}

type GetTaskPushNotificationConfigRequest struct {
	Name   string `json:"name"`
	Tenant string `json:"tenant"`
}

// Represents a request for the `tasks/get` method.
type GetTaskRequest struct {
	HistoryLength *int    `json:"historyLength,omitempty"`
	Name          string  `json:"name"`
	Tenant        *string `json:"tenant,omitempty"`
}

// Defines a security scheme using HTTP authentication.
type HTTPAuthSecurityScheme struct {
	BearerFormat *string `json:"bearerFormat,omitempty"`
	Description  *string `json:"description,omitempty"`
	Scheme       string  `json:"scheme"`
}

// Defines configuration details for the OAuth 2.0 Implicit flow.
type ImplicitOAuthFlow struct {
	AuthorizationURL string            `json:"authorizationUrl"`
	RefreshURL       *string           `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
}

type ListTaskPushNotificationConfigRequest struct {
	PageSize  int    `json:"pageSize"`
	PageToken string `json:"pageToken"`
	Parent    string `json:"parent"`
	Tenant    string `json:"tenant"`
}

// Represents a successful response for the `tasks/pushNotificationConfig/list`
// method.
type ListTaskPushNotificationConfigResponse struct {
	Configs       []TaskPushNotificationConfig `json:"configs,omitempty"`
	NextPageToken string                       `json:"nextPageToken"`
}

// Parameters for listing tasks with optional filtering criteria.
type ListTasksRequest struct {
	ContextID        string    `json:"contextId"`
	HistoryLength    *int      `json:"historyLength,omitempty"`
	IncludeArtifacts *bool     `json:"includeArtifacts,omitempty"`
	LastUpdatedAfter int       `json:"lastUpdatedAfter"`
	PageSize         *int      `json:"pageSize,omitempty"`
	PageToken        string    `json:"pageToken"`
	Status           TaskState `json:"status"`
	Tenant           string    `json:"tenant"`
}

// Result object for tasks/list method containing an array of tasks and pagination information.
type ListTasksResponse struct {
	NextPageToken string `json:"nextPageToken"`
	PageSize      int    `json:"pageSize"`
	Tasks         []Task `json:"tasks"`
	TotalSize     int    `json:"totalSize"`
}

// Message is one unit of communication between client and server. It is
// associated with a context and optionally a task. Since the server is
// responsible for the context definition, it must always provide a context_id
// in its messages. The client can optionally provide the context_id if it
// knows the context to associate the message to. Similarly for task_id,
// except the server decides if a task is created and whether to include the
// task_id.
type Message struct {
	ContextID        *string  `json:"contextId,omitempty"`
	Extensions       []string `json:"extensions,omitempty"`
	MessageID        string   `json:"messageId"`
	Metadata         *Struct  `json:"metadata,omitempty"`
	Parts            []Part   `json:"parts"`
	ReferenceTaskIds []string `json:"referenceTaskIds,omitempty"`
	Role             Role     `json:"role"`
	TaskID           *string  `json:"taskId,omitempty"`
}

// Defines a security scheme using mTLS authentication.
type MutualTlsSecurityScheme struct {
	Description string `json:"description"`
}

// Defines a security scheme using OAuth 2.0.
type OAuth2SecurityScheme struct {
	Description       *string    `json:"description,omitempty"`
	Flows             OAuthFlows `json:"flows"`
	Oauth2metadataURL *string    `json:"oauth2MetadataUrl,omitempty"`
}

// Defines the configuration for the supported OAuth 2.0 flows.
type OAuthFlows struct {
	AuthorizationCode *AuthorizationCodeOAuthFlow `json:"authorizationCode,omitempty"`
	ClientCredentials *ClientCredentialsOAuthFlow `json:"clientCredentials,omitempty"`
	Implicit          *ImplicitOAuthFlow          `json:"implicit,omitempty"`
	Password          *PasswordOAuthFlow          `json:"password,omitempty"`
}

// Defines a security scheme using OpenID Connect.
type OpenIdConnectSecurityScheme struct {
	Description      *string `json:"description,omitempty"`
	OpenIDConnectURL string  `json:"openIdConnectUrl"`
}

// Part represents a container for a section of communication content.
// Parts can be purely textual, some sort of file (image, video, etc) or
// a structured data blob (i.e. JSON).
type Part struct {
	Data     *DataPart `json:"data,omitempty"`
	File     *FilePart `json:"file,omitempty"`
	Metadata *Struct   `json:"metadata,omitempty"`
	Text     *string   `json:"text,omitempty"`
}

// Defines configuration details for the OAuth 2.0 Resource Owner Password flow.
type PasswordOAuthFlow struct {
	RefreshURL *string           `json:"refreshUrl,omitempty"`
	Scopes     map[string]string `json:"scopes"`
	TokenURL   string            `json:"tokenUrl"`
}

// Configuration for setting up push notifications for task updates.
type PushNotificationConfig struct {
	Authentication *AuthenticationInfo `json:"authentication,omitempty"`
	ID             *string             `json:"id,omitempty"`
	Token          *string             `json:"token,omitempty"`
	URL            string              `json:"url"`
}

type Security struct {
	Schemes map[string]StringList `json:"schemes,omitempty"`
}

// Defines a security scheme that can be used to secure an agent's endpoints.
// This is a discriminated union type based on the OpenAPI 3.2 Security Scheme Object.
// See: https://spec.openapis.org/oas/v3.2.0.html#security-scheme-object
type SecurityScheme struct {
	APIKeySecurityScheme        *APIKeySecurityScheme        `json:"apiKeySecurityScheme,omitempty"`
	HTTPAuthSecurityScheme      *HTTPAuthSecurityScheme      `json:"httpAuthSecurityScheme,omitempty"`
	MtlsSecurityScheme          *MutualTlsSecurityScheme     `json:"mtlsSecurityScheme,omitempty"`
	Oauth2securityScheme        *OAuth2SecurityScheme        `json:"oauth2SecurityScheme,omitempty"`
	OpenIDConnectSecurityScheme *OpenIdConnectSecurityScheme `json:"openIdConnectSecurityScheme,omitempty"`
}

// Configuration of a send message request.
type SendMessageConfiguration struct {
	AcceptedOutputModes    []string                `json:"acceptedOutputModes,omitempty"`
	Blocking               bool                    `json:"blocking"`
	HistoryLength          *int                    `json:"historyLength,omitempty"`
	PushNotificationConfig *PushNotificationConfig `json:"pushNotificationConfig,omitempty"`
}

// /////////// Request Messages ///////////
// Represents a request for the `message/send` method.
type SendMessageRequest struct {
	Configuration *SendMessageConfiguration `json:"configuration,omitempty"`
	Message       *Message                  `json:"message,omitempty"`
	Metadata      *Struct                   `json:"metadata,omitempty"`
	Tenant        string                    `json:"tenant"`
}

// ////// Response Messages ///////////
type SendMessageResponse struct {
	Message *Message `json:"message,omitempty"`
	Task    *Task    `json:"task,omitempty"`
}

// Represents a request for the `tasks/pushNotificationConfig/set` method.
type SetTaskPushNotificationConfigRequest struct {
	Config   TaskPushNotificationConfig `json:"config"`
	ConfigID string                     `json:"configId"`
	Parent   string                     `json:"parent"`
	Tenant   *string                    `json:"tenant,omitempty"`
}

// A wrapper object used in streaming operations to encapsulate different types of response data.
type StreamResponse struct {
	ArtifactUpdate *TaskArtifactUpdateEvent `json:"artifactUpdate,omitempty"`
	Message        *Message                 `json:"message,omitempty"`
	StatusUpdate   *TaskStatusUpdateEvent   `json:"statusUpdate,omitempty"`
	Task           *Task                    `json:"task,omitempty"`
}

// protolint:disable REPEATED_FIELD_NAMES_PLURALIZED
type StringList struct {
	List []string `json:"list,omitempty"`
}

type Struct = map[string]any

type SubscribeToTaskRequest struct {
	Name   string `json:"name"`
	Tenant string `json:"tenant"`
}

// Task is the core unit of action for A2A. It has a current status
// and when results are created for the task they are stored in the
// artifact. If there are multiple turns for a task, these are stored in
// history.
type Task struct {
	Artifacts []Artifact `json:"artifacts,omitempty"`
	ContextID string     `json:"contextId"`
	History   []Message  `json:"history,omitempty"`
	ID        string     `json:"id"`
	Metadata  *Struct    `json:"metadata,omitempty"`
	Status    TaskStatus `json:"status"`
}

// TaskArtifactUpdateEvent represents a task delta where an artifact has
// been generated.
type TaskArtifactUpdateEvent struct {
	Append    *bool    `json:"append,omitempty"`
	Artifact  Artifact `json:"artifact"`
	ContextID string   `json:"contextId"`
	LastChunk *bool    `json:"lastChunk,omitempty"`
	Metadata  *Struct  `json:"metadata,omitempty"`
	TaskID    string   `json:"taskId"`
}

// A container associating a push notification configuration with a specific
// task.
type TaskPushNotificationConfig struct {
	Name                   string                 `json:"name"`
	PushNotificationConfig PushNotificationConfig `json:"pushNotificationConfig"`
}

// A container for the status of a task
type TaskStatus struct {
	Message   *Message   `json:"message,omitempty"`
	State     TaskState  `json:"state"`
	Timestamp *Timestamp `json:"timestamp,omitempty"`
}

// An event sent by the agent to notify the client of a change in a task's
// status.
type TaskStatusUpdateEvent struct {
	ContextID string     `json:"contextId"`
	Final     bool       `json:"final"`
	Metadata  *Struct    `json:"metadata,omitempty"`
	Status    TaskStatus `json:"status"`
	TaskID    string     `json:"taskId"`
}

type Timestamp = time.Time
