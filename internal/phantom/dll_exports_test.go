package phantom

import (
	"debug/pe"
	"encoding/binary"
	"path/filepath"
	"testing"
)

func TestSteamAPI64DLLExports(t *testing.T) {
	dllPath := filepath.Join("..", "embedded", "bin", "windows", "steam_api64.dll")
	pf, err := pe.Open(dllPath)
	if err != nil {
		t.Fatalf("failed to open %s: %v", dllPath, err)
	}
	defer pf.Close()

	oh, ok := pf.OptionalHeader.(*pe.OptionalHeader64)
	if !ok {
		t.Fatal("expected PE32+ (64-bit) optional header")
	}
	exportDir := oh.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_EXPORT]
	if exportDir.VirtualAddress == 0 {
		t.Fatal("no export directory in DLL")
	}

	rvaToOffset := func(rva uint32) uint32 {
		for _, s := range pf.Sections {
			if rva >= s.VirtualAddress && rva < s.VirtualAddress+s.VirtualSize {
				return s.Offset + (rva - s.VirtualAddress)
			}
		}
		return 0
	}

	var secData []byte
	var secVA uint32
	for _, s := range pf.Sections {
		if exportDir.VirtualAddress >= s.VirtualAddress && exportDir.VirtualAddress < s.VirtualAddress+s.VirtualSize {
			secData, err = s.Data()
			if err != nil {
				t.Fatal(err)
			}
			secVA = s.VirtualAddress
			break
		}
	}
	if secData == nil {
		t.Fatal("could not locate section for export directory")
	}
	_ = rvaToOffset

	off := exportDir.VirtualAddress - secVA
	numNames := binary.LittleEndian.Uint32(secData[off+24 : off+28])
	namesRVA := binary.LittleEndian.Uint32(secData[off+32 : off+36])
	namesOff := namesRVA - secVA

	exports := make(map[string]bool, numNames)
	for i := uint32(0); i < numNames; i++ {
		nameRVA := binary.LittleEndian.Uint32(secData[namesOff+i*4 : namesOff+(i+1)*4])
		nOff := nameRVA - secVA
		end := nOff
		for end < uint32(len(secData)) && secData[end] != 0 {
			end++
		}
		exports[string(secData[nOff:end])] = true
	}

	for _, want := range []string{"SteamAPI_InitFlat", "SteamAPI_InitSafe", "SteamAPI_RunCallbacks", "SteamAPI_Shutdown"} {
		if !exports[want] {
			t.Errorf("steam_api64.dll missing required export %q", want)
		}
	}
}
