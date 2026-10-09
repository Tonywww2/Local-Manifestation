// Package expedition manages one floor's action-point balances and paid node
// admissions. It does not depend on rendering, networking, or a scripting VM.
//
// The server supplies the floor's initial values after applying layer, team,
// accessory, and metagame modifiers. Every player receives the same personal
// initial value; the team has one separate shared pool. A new Floor is created
// for each layer, so initial-value modifiers never rewrite an existing balance.
//
// Shared spending requires an injected authorizer; this package deliberately
// supplies no default voting policy. Approval to spend and participation are
// separate: the authorized proposal binds the floor, encounter, kind, and cost,
// while the entry lists the participating players.
//
// Encounter IDs identify individual visits, not reusable map-node definitions.
// The server remains responsible for map eligibility, revisits, late admission,
// boss assembly, player death, rewards, and actual combat. CompleteEncounter
// means that an encounter has finished settling, not necessarily that it was won.
// Boss routing is recorded independently from the combat outcome.
package expedition
