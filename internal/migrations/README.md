# Migrations

## Why migrations
The idea is to introduce a standardized process for internal changes of the
`Avalanche` tool which usually require scripting.

By running these migrations, we don't require external scripting, 
encapsulating different subsequent internal changes in sort of a 
managed, reproducible way which also represents its history.

The concept is obviously borrowed from databases where migrations
are being applied to update database structures and its data
to new versions of schemas and applications.

## Limitations
Usually migrations have a rollback path which can be applied in case of failures.
This tool currently does not support rollbacks.

## General structure
* The application calls all migrations implemnted when booting 
* Each migration is iterated in order (the order is "enforced" via the index in the migrations map)
* Each migration checks itself if it needs to be applied
* The first migration which is getting applied prints a message to the user
* At the end of the iteration, it is checked if any migration ran. If one did, a closing message is printed
* If no migration ran at all, nothing is being printed

## Implementation
* Migrations need to reside in `internal/migrations`
* Each new migration should be added into a separate file in the package
* Each new migration needs to implement a `migrationFunc`
* It adds itself to the global `migrations` map with the next available index
* Each new migration needs to check itself if it needs to be applied depending on what it does
* If it needs to be applied, it should run `printMigrationMessage` as an info to the user
* Otherwise it doesn't need to print anything (to not add confusion)
* Every migration is expected to clean after itself if needed
