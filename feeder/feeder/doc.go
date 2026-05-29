// Package feeder implements the vertix-feeder oracle sidecar: it fetches
// external prices, computes a quality-gated cross-source median per pair, and
// broadcasts one batched MsgSubmitFeed transaction per oracle vote window.
package feeder
