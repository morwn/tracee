package derive

import (
	"testing"

	"github.com/aquasecurity/tracee/pkg/utils/sharedobjs"
	"github.com/aquasecurity/tracee/types/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type soInstance struct {
	info sharedobjs.ObjInfo
	syms []string
}

type symbolsLoaderMock struct {
	cache map[sharedobjs.ObjInfo]map[string]bool
}

func initLoaderMock() symbolsLoaderMock {
	return symbolsLoaderMock{cache: make(map[sharedobjs.ObjInfo]map[string]bool)}
}

func (loader symbolsLoaderMock) GetDynamicSymbols(info sharedobjs.ObjInfo) (map[string]bool, error) {
	return loader.cache[info], nil
}

func (loader symbolsLoaderMock) GetExportedSymbols(info sharedobjs.ObjInfo) (map[string]bool, error) {
	return loader.cache[info], nil
}

func (loader symbolsLoaderMock) GetImportedSymbols(info sharedobjs.ObjInfo) (map[string]bool, error) {
	return nil, nil
}

func (loader symbolsLoaderMock) addSOSymbols(info soInstance) {
	symsMap := make(map[string]bool)
	for _, s := range info.syms {
		symsMap[s] = true
	}
	loader.cache[info.info] = symsMap
}

func generateSOLoadedEvent(pid int, so sharedobjs.ObjInfo) trace.Event {
	return trace.Event{
		EventName:     "shared_object_loaded",
		EventID:       1036,
		HostProcessID: pid,
		ProcessID:     pid,
		Args: []trace.Argument{
			{ArgMeta: trace.ArgMeta{Type: "const char*", Name: "pathname"}, Value: so.Path},
			{ArgMeta: trace.ArgMeta{Type: "int", Name: "flags"}, Value: 0},
			{ArgMeta: trace.ArgMeta{Type: "dev_t", Name: "dev"}, Value: so.Id.Device},
			{ArgMeta: trace.ArgMeta{Type: "unsigned long", Name: "inode"}, Value: so.Id.Inode},
			{ArgMeta: trace.ArgMeta{Type: "unsigned long", Name: "ctime"}, Value: so.Id.Ctime},
		},
	}
}

func TestDeriveSharedObjectExportWatchedSymbols(t *testing.T) {
	testCases := []struct {
		name            string
		watchedSymbols  []string
		whitelistedLibs []string
		loadingSO       soInstance
		expectedSymbols []string
	}{
		{
			name:            "SO with no export symbols",
			watchedSymbols:  []string{"open", "close", "write"},
			whitelistedLibs: []string{},
			loadingSO: soInstance{
				info: sharedobjs.ObjInfo{Id: sharedobjs.ObjID{Inode: 1}, Path: "1.so"},
				syms: []string{},
			},
			expectedSymbols: []string{},
		},
		{
			name:            "SO with 1 watched export symbols",
			watchedSymbols:  []string{"open", "close", "write"},
			whitelistedLibs: []string{},
			loadingSO: soInstance{
				info: sharedobjs.ObjInfo{Id: sharedobjs.ObjID{Inode: 1}, Path: "1.so"},
				syms: []string{"open"},
			},
			expectedSymbols: []string{"open"},
		},
		{
			name:            "SO with multiple watched export symbols",
			watchedSymbols:  []string{"open", "close", "write"},
			whitelistedLibs: []string{},
			loadingSO: soInstance{
				info: sharedobjs.ObjInfo{Id: sharedobjs.ObjID{Inode: 1}, Path: "1.so"},
				syms: []string{
					"open",
					"close",
					"write",
				},
			},
			expectedSymbols: []string{"open", "close", "write"},
		},
		{
			name:            "SO with partly watched export symbols",
			watchedSymbols:  []string{"open", "close", "write"},
			whitelistedLibs: []string{},
			loadingSO: soInstance{
				info: sharedobjs.ObjInfo{Id: sharedobjs.ObjID{Inode: 1}, Path: "1.so"},
				syms: []string{
					"open",
					"close",
					"sync",
				},
			},
			expectedSymbols: []string{"open", "close"},
		},
		{
			name:            "SO with no watched export symbols",
			watchedSymbols:  []string{"open", "close", "write"},
			whitelistedLibs: []string{},
			loadingSO: soInstance{
				info: sharedobjs.ObjInfo{Id: sharedobjs.ObjID{Inode: 1}, Path: "1.so"},
				syms: []string{
					"createdir",
					"rmdir",
					"walk",
				},
			},
			expectedSymbols: []string{},
		},
		{
			name:            "whitelisted full path SO",
			watchedSymbols:  []string{"open", "close", "write"},
			whitelistedLibs: []string{"/tmp/test"},
			loadingSO: soInstance{
				info: sharedobjs.ObjInfo{Id: sharedobjs.ObjID{Inode: 1}, Path: "/tmp/test.so"},
				syms: []string{"open"},
			},
			expectedSymbols: []string{},
		},
		{
			name:            "whitelisted SO name",
			watchedSymbols:  []string{"open", "close", "write"},
			whitelistedLibs: []string{"test"},
			loadingSO: soInstance{
				info: sharedobjs.ObjInfo{Id: sharedobjs.ObjID{Inode: 1}, Path: "/lib/test.so"},
				syms: []string{"open"},
			},
			expectedSymbols: []string{},
		},
	}
	pid := 1

	t.Run("UT", func(t *testing.T) {
		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				mockLoader := initLoaderMock()
				mockLoader.addSOSymbols(testCase.loadingSO)
				gen := initSymbolsLoadedEventGenerator(mockLoader, testCase.watchedSymbols, testCase.whitelistedLibs)
				eventArgs, err := gen.deriveArgs(generateSOLoadedEvent(pid, testCase.loadingSO.info))
				require.NoError(t, err)
				if len(testCase.expectedSymbols) > 0 {
					require.Len(t, eventArgs, 2)
					path := eventArgs[0]
					syms := eventArgs[1]
					require.IsType(t, "", path)
					require.IsType(t, []string{}, syms)
					assert.ElementsMatch(t, testCase.expectedSymbols, syms.([]string))
					assert.Equal(t, testCase.loadingSO.info.Path, path.(string))
				} else {
					assert.Len(t, eventArgs, 0)
				}
			})
		}
	})
}
