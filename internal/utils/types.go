package utils

import pb "go.protobuf.mentenova.exchange/mentenova/db/resources/bigtable/v1"

// ProviderClients is a container for all provider clients.
type ProviderClients struct {
	Bigtable pb.BigtableServiceClient
	Spanner  pb.SpannerServiceClient
}
