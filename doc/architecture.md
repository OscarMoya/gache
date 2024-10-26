# gache Architecture

The goal of this project is to deliver a cache to be used as an in-memory key-value store or as a remote cache. Using maps for caches is a wonderful option when the data to put in the cache is small and the footprint of the reset of the application is bigger compared to the data flows using the cache. However, when the data to cache is big, the GC strain can be a problem, especially using maps. Maps as dynamically allocated memory structures are not guaranteed to be contiguous in memory, additionally if the map values are pointers, the GC will have to scan the pointers to check if they are still in use. This can lead to a lot of GC pauses and a lot of memory fragmentation, Making the application slower and spiking each time the GC cycles run.

## Cache Types

For simplicity, the cache is not going to behave as a generic cache, for the first versions is not going to support any eviction policy, other than the TTL. The cache is going to be a key-value store, where both the key and the value are going to be `[]byte`.


## Memory Management

For the first versions, the cache is going to pre-allocate the memory for the cache, avoiding any kind of dynamically allocated memory, to prevent spikes in the GC. Later on, it will be evaluated how to resize data if needed with minimal footprint.

## Key Components

The cache is going to be composed of the following components:

* Cache: The cache is going to be the main component, it is going to be the one that is going to be used to interact with the cache and will offeer an API similar to a map.
* Shard(s): These are going to be where the data is going to be stored. The cache is going to be composed of multiple shards, where it shard will be chosen based on the    key hash.
* Hasher: The hasher is the function to be used to hash the keys to choose the shard or to use as storage. The hash function is key to the performance of the cache, as it is going to be used to distribute the data across the shards. However, it is outside the scope of this project to provide a good hash function and default one, trying to get the best out there, is going to be used.


![GacheBase](/assets/GacheBase.svg)


## Shard Data Structure

The shard data struct is going to use structs as that are continuous in memory as much as possible, and if not, the there types to be used will be simple types, as uints, or similar.