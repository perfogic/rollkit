package types

// Metadata key constants used across the rollkit codebase.
// These keys are used to store various metadata in the store.
const (
	// DAIncludedHeightKey is the key used for persisting the DA included height in store.
	// This represents the height of the data availability layer that has been included.
	DAIncludedHeightKey = "d"

	// LastBatchDataKey is the key used for persisting the last batch data in store.
	// This contains the last batch data submitted to the data availability layer.
	LastBatchDataKey = "l"

	// LastSubmittedHeaderHeightKey is the key used for persisting the last submitted header height in store.
	// This represents the height of the last header submitted to DA.
	LastSubmittedHeaderHeightKey = "last-submitted-header-height"

	// LastSubmittedDataHeightKey is the key used for persisting the last submitted data height in store.
	// This represents the height of the last data submitted to DA.
	LastSubmittedDataHeightKey = "last-submitted-data-height"
)

// GetKnownMetadataKeys returns a map of all known metadata keys with their descriptions.
func GetKnownMetadataKeys() map[string]string {
	return map[string]string{
		DAIncludedHeightKey:         "DA included height - the height of the data availability layer that has been included",
		LastBatchDataKey:            "Last batch data - the last batch data submitted to the data availability layer",
		LastSubmittedHeaderHeightKey: "Last submitted header height - the height of the last header submitted to DA",
		LastSubmittedDataHeightKey:   "Last submitted data height - the height of the last data submitted to DA",
	}
}

// GetKnownMetadataKeysList returns a slice of all known metadata keys.
func GetKnownMetadataKeysList() []string {
	return []string{
		DAIncludedHeightKey,
		LastBatchDataKey,
		LastSubmittedHeaderHeightKey,
		LastSubmittedDataHeightKey,
	}
}