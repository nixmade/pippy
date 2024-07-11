package store

type ValueIterator func(any, any) error

// Store provides a way for defining multiple stores
type Store interface {
	SaveJSON(key string, value interface{}) error                                      // Save key json value to store, returns error on failure
	Delete(key string) error                                                           // Delete key from store, returns error on failure
	LoadJSON(key string, value interface{}) error                                      // Load key from store, unmarshals json value, returns error on failure
	LoadKeys(prefix string) ([]string, error)                                          // Load all keys from store, returns error on failure
	LoadValues(prefix string, iter ValueIterator) error                                // Loads all keys and values from store, return error on failure
	Count(prefix string) (uint64, error)                                               // returns count of specified prefix, or error on failure
	CountJsonPath(prefix, jsonPath string, iter ValueIterator) error                   // returns grouped count of jsonpath, returns error on failure
	QueryJsonPath(prefix, jsonPath string, iter ValueIterator) error                   // returns key and value of jsonpath, returns error on failure
	SortedAscN(prefix string, jsonPath string, limit int64, iter ValueIterator) error  // returns N key values, sorted ascending order by jsonpath, returns error on failure
	SortedDescN(prefix string, jsonPath string, limit int64, iter ValueIterator) error // returns N key values, sorted descending order by jsonpath, returns error on failure
	DeletePrefix(prefix string) error                                                  // Delete prefix pattern from store, returns error on failure
	Close() error
}
