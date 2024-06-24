package conftypes

import (
	"reflect"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	proto "github.com/sourcegraph/sourcegraph/internal/api/internalapi/v1"
)

// ServiceConnections represents configuration about how the deployment
// internally connects to services. These are settings that need to be
// propagated from the frontend to other services, so that the frontend
// can be the source of truth for all configuration.
type ServiceConnections struct {
	// GitServers is the addresses of gitserver instances that should be
	// talked to.
	GitServers []string `json:"gitServers"`

	// PostgresDSN is the PostgreSQL DB data source name.
	// eg: "postgres://sg@pgsql/sourcegraph?sslmode=false"
	PostgresDSN string `json:"postgresDSN"`

	// CodeIntelPostgresDSN is the PostgreSQL DB data source name for the
	// code intel database.
	// eg: "postgres://sg@pgsql/sourcegraph_codeintel?sslmode=false"
	CodeIntelPostgresDSN string `json:"codeIntelPostgresDSN"`

	// CodeInsightsDSN is the PostgreSQL DB data source name for the
	// code insights database.
	// eg: "postgres://sg@pgsql/sourcegraph_codeintel?sslmode=false"
	CodeInsightsDSN string `json:"codeInsightsPostgresDSN"`

	// Searchers is the addresses of searcher instances that should be talked to.
	Searchers []string `json:"searchers"`
	// Symbols is the addresses of symbol instances that should be talked to.
	Symbols []string `json:"symbols"`
	// Zoekts is the addresses of Zoekt instances to talk to.
	Zoekts []string `json:"zoekts"`
	// ZoektListTTL is the TTL of the internal cache that Zoekt clients use to
	// cache the list of indexed repository. After TTL is over, new list will
	// get requested from Zoekt shards.
	ZoektListTTL time.Duration `json:"zoektListTTL"`
}

func (sc *ServiceConnections) ToProto() *proto.ServiceConnections {
	return &proto.ServiceConnections{
		GitServers:           sc.GitServers,
		PostgresDsn:          sc.PostgresDSN,
		CodeIntelPostgresDsn: sc.CodeIntelPostgresDSN,
		CodeInsightsDsn:      sc.CodeInsightsDSN,
		Searchers:            sc.Searchers,
		Symbols:              sc.Symbols,
		Zoekts:               sc.Zoekts,
		ZoektListTtl:         durationpb.New(sc.ZoektListTTL),
	}
}

func (sc *ServiceConnections) FromProto(in *proto.ServiceConnections) {
	*sc = ServiceConnections{
		GitServers:           in.GetGitServers(),
		PostgresDSN:          in.GetPostgresDsn(),
		CodeIntelPostgresDSN: in.GetCodeIntelPostgresDsn(),
		CodeInsightsDSN:      in.GetCodeInsightsDsn(),
		Searchers:            in.GetSearchers(),
		Symbols:              in.GetSymbols(),
		Zoekts:               in.GetZoekts(),
		ZoektListTTL:         in.GetZoektListTtl().AsDuration(),
	}
}

// RawUnified is the unparsed variant of conf.Unified.
type RawUnified struct {
	ID                 int32
	Site               string
	ServiceConnections ServiceConnections
}

func (r *RawUnified) ToProto() *proto.RawUnified {
	return &proto.RawUnified{
		Id:                 r.ID,
		Site:               r.Site,
		ServiceConnections: r.ServiceConnections.ToProto(),
	}
}

func (r *RawUnified) FromProto(in *proto.RawUnified) {
	*r = RawUnified{
		ID:   in.GetId(),
		Site: in.GetSite(),
	}
	r.ServiceConnections.FromProto(in.GetServiceConnections())
}

// Equal tells if the two configurations are equal or not.
func (r RawUnified) Equal(other RawUnified) bool {
	return r.Site == other.Site && reflect.DeepEqual(r.ServiceConnections, other.ServiceConnections)
}

// Bedrock Model IDs can be in one of two forms:
//   - A static model ID, e.g. "anthropic.claude-v2".
//   - A model ID and ARN for provisioned capacity, e.g.
//     "anthropic.claude-v2/arn:aws:bedrock:us-west-2:012345678901:provisioned-model/xxxxxxxx"
//
// See the AWS docs for more information:
// https://docs.aws.amazon.com/bedrock/latest/userguide/model-ids.html
// https://docs.aws.amazon.com/bedrock/latest/APIReference/API_CreateProvisionedModelThroughput.html
type BedrockModelRef struct {
	// Model is the underlying LLM model Bedrock is serving, e.g. "anthropic.claude-3-haiku-20240307-v1:0
	Model string
	// If the configuration is using provisioned capacity, this will
	// contain the ARN of the model to use for making API calls.
	// e.g. "anthropic.claude-v2/arn:aws:bedrock:us-west-2:012345678901:provisioned-model/xxxxxxxx"
	ProvisionedCapacity *string
}

func NewBedrockModelRefFromModelID(modelID string) BedrockModelRef {
	parts := strings.SplitN(modelID, "/", 2)

	if parts == nil { // this shouldn't really happen
		return BedrockModelRef{Model: modelID}
	}

	parsed := BedrockModelRef{
		Model: parts[0],
	}

	if len(parts) == 2 {
		parsed.ProvisionedCapacity = &parts[1]
	}
	return parsed
}

// Ensures that all case insensitive parts of the model ID are lowercased so
// that they can be compared.
func (bmr BedrockModelRef) CanonicalizedModelID() string {
	// Bedrock models are case sensitive if they contain a ARN
	// make sure to only lowercase the non ARN part
	model := strings.ToLower(bmr.Model)

	if bmr.ProvisionedCapacity != nil {
		return strings.Join([]string{model, *bmr.ProvisionedCapacity}, "/")
	}
	return model
}
