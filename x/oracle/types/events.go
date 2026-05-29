package types

const (
	EventTypeFeederSet       = "oracle_feeder_set"
	EventTypeFeedSubmitted   = "oracle_feed_submitted"
	EventTypePriceAggregated = "oracle_price_aggregated"
	EventTypeSlash           = "oracle_slash"
	EventTypeParamsUpdated   = "oracle_params_updated"

	AttributeKeyValidator         = "validator"
	AttributeKeyFeeder            = "feeder"
	AttributeKeyPair              = "pair"
	AttributeKeyPrice             = "price"
	AttributeKeySlashReason       = "slash_reason"
	AttributeKeySlashFraction     = "slash_fraction"
	AttributeKeyAcceptListChanged = "accept_list_changed"
)
