/*
Package sfnnue is a Go port of Stockfish's NNUE evaluation.

This code is derived from Stockfish, a UCI chess playing engine.
Copyright (C) 2004-2026 The Stockfish developers (see AUTHORS file)

Stockfish is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Stockfish is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.

Original C++ source: https://github.com/official-stockfish/Stockfish

# Architecture

This package implements Stockfish's NNUE (Efficiently Updatable Neural Network)
evaluation function. The network uses a HalfKAv2_hm feature set with horizontal
mirroring, dual networks (big and small), and 8 layer stacks selected by piece count.

# Usage

	eval, err := sfnnue.NewEvaluator("nn-xxx.nnue")
	if err != nil {
		log.Fatal(err)
	}

	score := eval.Evaluate(position)
*/
package sfnnue
