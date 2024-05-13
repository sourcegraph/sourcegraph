package stats

// func TestCommitGraphInfoForRepository(t *testing.T) {
// 	t.Parallel()

// 	for _, tc := range []struct {
// 		desc         string
// 		setup        func(t *testing.T, repoPath string)
// 		expectedErr  error
// 		expectedInfo CommitGraphInfo
// 	}{
// 		{
// 			desc:         "no commit graph filter",
// 			setup:        func(*testing.T, string) {},
// 			expectedInfo: CommitGraphInfo{},
// 		},
// 		{
// 			desc: "single commit graph without bloom filter and generation data",
// 			setup: func(t *testing.T, repoPath string) {
// 				gittest.Exec(t, cfg, "-C", repoPath,
// 					"-c", "commitGraph.generationVersion=1",
// 					"commit-graph", "write", "--reachable",
// 				)
// 			},
// 			expectedInfo: CommitGraphInfo{
// 				Exists: true,
// 			},
// 		},
// 		{
// 			desc: "single commit graph with bloom filter",
// 			setup: func(t *testing.T, repoPath string) {
// 				gittest.Exec(t, cfg, "-C", repoPath,
// 					"-c", "commitGraph.generationVersion=1",
// 					"commit-graph", "write", "--reachable", "--changed-paths",
// 				)
// 			},
// 			expectedInfo: CommitGraphInfo{
// 				Exists:          true,
// 				HasBloomFilters: true,
// 			},
// 		},
// 		{
// 			desc: "single commit graph with generation numbers",
// 			setup: func(t *testing.T, repoPath string) {
// 				gittest.Exec(t, cfg, "-C", repoPath,
// 					"-c", "commitGraph.generationVersion=2",
// 					"commit-graph", "write", "--reachable", "--changed-paths",
// 				)
// 			},
// 			expectedInfo: CommitGraphInfo{
// 				Exists:            true,
// 				HasBloomFilters:   true,
// 				HasGenerationData: true,
// 			},
// 		},
// 		{
// 			desc: "split commit graph without bloom filter and generation data",
// 			setup: func(t *testing.T, repoPath string) {
// 				gittest.Exec(t, cfg, "-C", repoPath,
// 					"-c", "commitGraph.generationVersion=1",
// 					"commit-graph", "write", "--reachable", "--split",
// 				)
// 			},
// 			expectedInfo: CommitGraphInfo{
// 				Exists:                 true,
// 				CommitGraphChainLength: 1,
// 			},
// 		},
// 		{
// 			desc: "split commit graph with bloom filter without generation data",
// 			setup: func(t *testing.T, repoPath string) {
// 				gittest.Exec(t, cfg, "-C", repoPath,
// 					"-c", "commitGraph.generationVersion=1",
// 					"commit-graph", "write", "--reachable", "--split", "--changed-paths",
// 				)
// 			},
// 			expectedInfo: CommitGraphInfo{
// 				Exists:                 true,
// 				CommitGraphChainLength: 1,
// 				HasBloomFilters:        true,
// 			},
// 		},
// 		{
// 			desc: "split commit-graph with generation numbers",
// 			setup: func(t *testing.T, repoPath string) {
// 				gittest.Exec(t, cfg, "-C", repoPath,
// 					"-c", "commitGraph.generationVersion=2",
// 					"commit-graph", "write", "--reachable", "--split", "--changed-paths",
// 				)
// 			},
// 			expectedInfo: CommitGraphInfo{
// 				Exists:                 true,
// 				CommitGraphChainLength: 1,
// 				HasBloomFilters:        true,
// 				HasGenerationData:      true,
// 			},
// 		},
// 		{
// 			desc: "split commit-graph with generation data overflow",
// 			setup: func(t *testing.T, repoPath string) {
// 				// We write two commits, where the parent commit is far away in the
// 				// future and its child commit is in the past. This means we'll have
// 				// to write a corrected committer date, and because the corrected
// 				// date is longer than 31 bits we'll have to also write overflow
// 				// data.
// 				futureParent := gittest.WriteCommit(t, cfg, repoPath,
// 					gittest.WithCommitterDate(time.Date(2077, 1, 1, 0, 0, 0, 0, time.UTC)),
// 				)
// 				gittest.WriteCommit(t, cfg, repoPath,
// 					gittest.WithBranch("overflow"),
// 					gittest.WithParents(futureParent),
// 					gittest.WithCommitterDate(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)),
// 				)

// 				gittest.Exec(t, cfg, "-C", repoPath,
// 					"-c", "commitGraph.generationVersion=2",
// 					"commit-graph", "write", "--reachable", "--split", "--changed-paths",
// 				)
// 			},
// 			expectedInfo: CommitGraphInfo{
// 				Exists:                    true,
// 				CommitGraphChainLength:    1,
// 				HasBloomFilters:           true,
// 				HasGenerationData:         true,
// 				HasGenerationDataOverflow: true,
// 			},
// 		},
// 	} {
// 		t.Run(tc.desc, func(t *testing.T) {
// 			_, repoPath := gittest.CreateRepository(t, ctx, cfg, gittest.CreateRepositoryConfig{
// 				SkipCreationViaService: true,
// 			})
// 			gittest.WriteCommit(t, cfg, repoPath, gittest.WithBranch("main"))
// 			tc.setup(t, repoPath)

// 			info, err := CommitGraphInfoForRepository(repoPath)
// 			require.Equal(t, tc.expectedErr, err)
// 			require.Equal(t, tc.expectedInfo, info)
// 		})
// 	}
// }
