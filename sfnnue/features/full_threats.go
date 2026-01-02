// FullThreats feature set for NNUE evaluation (big network only).
// Ported from Stockfish src/nnue/features/full_threats.h and .cpp
//
// This feature captures threat relationships between pieces.

package features

// FullThreats feature constants

// Feature name (full_threats.h:39)
const ThreatName = "Full_Threats(Friend)"

// Hash value embedded in the evaluation file (full_threats.h:42)
const ThreatHashValue uint32 = 0x8f234cb8

// Number of feature dimensions (full_threats.h:45)
const ThreatDimensions = 79856

// Maximum number of simultaneously active features (full_threats.h:78)
const ThreatMaxActiveDimensions = 128

// Number of valid targets for each piece type (full_threats.h:33-34)
var NumValidTargets = [PIECE_NB]int{
	0, 6, 12, 10, 10, 12, 8, 0,
	0, 6, 12, 10, 10, 12, 8, 0,
}

// ThreatOrientTBL orients a square for threat features (full_threats.h:49-58)
// Note: This is different from HalfKAv2_hm's OrientTBL
var ThreatOrientTBL = [SQUARE_NB]int{
	SQ_A1, SQ_A1, SQ_A1, SQ_A1, SQ_H1, SQ_H1, SQ_H1, SQ_H1,
	SQ_A1, SQ_A1, SQ_A1, SQ_A1, SQ_H1, SQ_H1, SQ_H1, SQ_H1,
	SQ_A1, SQ_A1, SQ_A1, SQ_A1, SQ_H1, SQ_H1, SQ_H1, SQ_H1,
	SQ_A1, SQ_A1, SQ_A1, SQ_A1, SQ_H1, SQ_H1, SQ_H1, SQ_H1,
	SQ_A1, SQ_A1, SQ_A1, SQ_A1, SQ_H1, SQ_H1, SQ_H1, SQ_H1,
	SQ_A1, SQ_A1, SQ_A1, SQ_A1, SQ_H1, SQ_H1, SQ_H1, SQ_H1,
	SQ_A1, SQ_A1, SQ_A1, SQ_A1, SQ_H1, SQ_H1, SQ_H1, SQ_H1,
	SQ_A1, SQ_A1, SQ_A1, SQ_A1, SQ_H1, SQ_H1, SQ_H1, SQ_H1,
}

// ThreatMap maps attacker type to attacked type feature index (full_threats.h:60-67)
// -1 means excluded pair
var ThreatMap = [6][6]int{
	{0, 1, -1, 2, -1, -1}, // Pawn attacks
	{0, 1, 2, 3, 4, 5},    // Knight attacks
	{0, 1, 2, 3, -1, 4},   // Bishop attacks
	{0, 1, 2, 3, -1, 4},   // Rook attacks
	{0, 1, 2, 3, 4, 5},    // Queen attacks
	{0, 1, 2, 3, -1, -1},  // King attacks
}

// DirtyThreats represents changed threat features for incremental updates.
type DirtyThreats struct {
	Us      int // Side that moved
	Ksq     int // Current king square
	PrevKsq int // Previous king square
	List    []ThreatEntry
}

// ThreatEntry represents a single threat change
type ThreatEntry struct {
	Attacker     int  // Attacking piece
	AttackerSq   int  // Attacker square
	Attacked     int  // Attacked piece
	AttackedSq   int  // Attacked square
	IsAddition   bool // True if adding, false if removing
}

// Pc returns the attacking piece
func (t *ThreatEntry) Pc() int { return t.Attacker }

// ThreatenedPc returns the attacked piece
func (t *ThreatEntry) ThreatenedPc() int { return t.Attacked }

// PcSq returns the attacker square
func (t *ThreatEntry) PcSq() int { return t.AttackerSq }

// ThreatenedSq returns the attacked square
func (t *ThreatEntry) ThreatenedSq() int { return t.AttackedSq }

// Add returns whether this is an addition
func (t *ThreatEntry) Add() bool { return t.IsAddition }

// ThreatRequiresRefresh returns whether the change means a full accumulator refresh is required.
// Ported from full_threats.cpp:357-359
func ThreatRequiresRefresh(diff *DirtyThreats, perspective int) bool {
	// Refresh if king moved across the e-file boundary (bit 2 of file changed)
	return perspective == diff.Us && (diff.Ksq&0b100) != (diff.PrevKsq&0b100)
}

// ThreatIndexList is a list of threat feature indices
type ThreatIndexList struct {
	Values [ThreatMaxActiveDimensions]int
	Size   int
}

// Push adds an index to the list
func (l *ThreatIndexList) Push(idx int) {
	if l.Size < ThreatMaxActiveDimensions {
		l.Values[l.Size] = idx
		l.Size++
	}
}

// Clear resets the list
func (l *ThreatIndexList) Clear() {
	l.Size = 0
}
