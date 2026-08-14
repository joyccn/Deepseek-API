package pow

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

//go:embed sha3_wasm_bg.wasm
var wasmBytes []byte

type Solver struct {
	runtime wazero.Runtime
	module  api.Module
	mu      sync.Mutex
}

func NewSolver(ctx context.Context) (*Solver, error) {
	r := wazero.NewRuntime(ctx)
	mod, err := r.Instantiate(ctx, wasmBytes)
	if err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASM module: %w", err)
	}
	return &Solver{
		runtime: r,
		module:  mod,
	}, nil
}

func (s *Solver) Close(ctx context.Context) error {
	return s.runtime.Close(ctx)
}

func (s *Solver) MakeHeader(ctx context.Context, challenge map[string]any) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	salt, _ := challenge["salt"].(string)
	var expireAtStr string
	switch v := challenge["expire_at"].(type) {
	case float64:
		expireAtStr = fmt.Sprintf("%.0f", v)
	case int64:
		expireAtStr = fmt.Sprintf("%d", v)
	case int:
		expireAtStr = fmt.Sprintf("%d", v)
	case string:
		expireAtStr = v
	default:
		expireAtStr = fmt.Sprintf("%v", v)
	}
	prefix := fmt.Sprintf("%s_%s_", salt, expireAtStr)
	chStr, _ := challenge["challenge"].(string)

	var diff float64
	switch v := challenge["difficulty"].(type) {
	case float64:
		diff = v
	case float32:
		diff = float64(v)
	case int64:
		diff = float64(v)
	case int:
		diff = float64(v)
	case json.Number:
		diff, _ = v.Float64()
	case string:
		fmt.Sscanf(v, "%f", &diff)
	}

	if ctx == nil {
		ctx = context.Background()
	}

	ans, err := s.solve(ctx, chStr, prefix, diff)
	if err != nil {
		return "", err
	}

	payload := map[string]any{
		"algorithm":   challenge["algorithm"],
		"challenge":   challenge["challenge"],
		"salt":        challenge["salt"],
		"answer":      ans,
		"signature":   challenge["signature"],
		"target_path": challenge["target_path"],
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func (s *Solver) solve(ctx context.Context, challenge, prefix string, difficulty float64) (int64, error) {
	malloc := s.module.ExportedFunction("__wbindgen_export_0")
	addStack := s.module.ExportedFunction("__wbindgen_add_to_stack_pointer")
	wasmSolve := s.module.ExportedFunction("wasm_solve")

	if malloc == nil || addStack == nil || wasmSolve == nil {
		return 0, fmt.Errorf("missing required exported functions in WASM")
	}

	// Allocate shadow stack (-16 bytes: 64-bit sign-extended)
	shift := int64(-16)
	retPtrRes, err := addStack.Call(ctx, uint64(shift))
	if err != nil {
		return 0, fmt.Errorf("stack alloc failed: %w", err)
	}
	retPtr := uint32(retPtrRes[0])

	defer addStack.Call(ctx, uint64(16))

	// Allocate strings
	cBytes := []byte(challenge)
	cPtrRes, err := malloc.Call(ctx, uint64(len(cBytes)), 1)
	if err != nil {
		return 0, fmt.Errorf("malloc challenge string failed: %w", err)
	}
	cPtr := uint32(cPtrRes[0])
	s.module.Memory().Write(cPtr, cBytes)

	pBytes := []byte(prefix)
	pPtrRes, err := malloc.Call(ctx, uint64(len(pBytes)), 1)
	if err != nil {
		return 0, fmt.Errorf("malloc prefix string failed: %w", err)
	}
	pPtr := uint32(pPtrRes[0])
	s.module.Memory().Write(pPtr, pBytes)

	_, err = wasmSolve.Call(ctx,
		uint64(retPtr),
		uint64(cPtr), uint64(len(cBytes)),
		uint64(pPtr), uint64(len(pBytes)),
		math.Float64bits(difficulty),
	)
	if err != nil {
		return 0, fmt.Errorf("wasm_solve execution failed: %w", err)
	}

	mem, ok := s.module.Memory().Read(retPtr, 16)
	if !ok || len(mem) < 16 {
		return 0, fmt.Errorf("failed to read WASM return pointer memory")
	}

	status := int32(binary.LittleEndian.Uint32(mem[0:4]))
	if status == 0 {
		return 0, fmt.Errorf("wasm_solve returned status 0 (challenge failed or expired)")
	}

	valBits := binary.LittleEndian.Uint64(mem[8:16])
	val := math.Float64frombits(valBits)
	return int64(val), nil
}
