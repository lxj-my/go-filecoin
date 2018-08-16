// Package api holds the interface definitions for the Filecoin api.
package api

// API is the user interface to a Filecoin node.
type API interface {
	Actor() Actor
	Address() Address
	Block() Block
	Bootstrap() Bootstrap
	Chain() Chain
	Config() Config
	Client() Client
	Daemon() Daemon
	Dag() Dag
	ID() ID
	Log() Log
	Message() Message
	Miner() Miner
	Mining() Mining
	Mpool() Mpool
	Orderbook() Orderbook
	Paych() Paych
	Ping() Ping
	Swarm() Swarm
	Version() Version
}