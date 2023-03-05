package usagestats

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestMigratedExtensionsUsageStatistics(t *testing.T) {
	ctx := context.Background()

	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(logger, t))

	_, err := db.ExecContext(ctx, `
		INSERT INTO event_logs
			(id, name, url, user_id, anonymous_user_id, source, argument, version, "timestamp", feature_flags, cohort_id, public_argument, first_source_url, last_source_url, referrer, device_id, insert_id)
		VALUES
			(9314, 'GitBlameEnabled', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go?L48:2', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 13:54:47.919789+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '99dcb7b6-417d-4146-b98d-d7dea77f7aa1'),
			(9315, 'GitBlameDisabled', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go?L48:2', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 13:54:47.91979+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '49577819-6140-4b3e-8f79-55dceab5992d'),
			(9316, 'GitBlameEnabled', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go?L48:2', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 13:54:50.195024+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '2c2d26f7-f068-418f-b676-1dadc464442f'),
			(9317, 'GitBlamePopupViewed', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go?L48:2', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 13:56:12.153929+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'da5c5613-95f1-4ed9-9205-f883cd015456'),
			(9319, 'GitBlameEnabled', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go?L48:2', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 13:56:36.093035+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '487235d4-7a4b-443d-b89a-a19152844f6f'),
			(9320, 'GitBlamePopupViewed', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go?L48:2', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 13:56:38.380018+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '0a9a962f-ed6a-4120-b63a-64b7a0edd6c4'),
			(9321, 'GitBlamePopupViewed', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go?L48:2', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 13:56:38.380018+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '6ce1c1a0-d7e9-40d5-956a-abacc6018d5d'),
			(9332, 'GitBlamePopupClicked', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go?L48:2', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 13:57:26.752507+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'e07e9fb1-9662-4172-bf44-80a458c81da2'),
			(9335, 'GitBlamePopupClicked', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go?L48:2', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 13:57:32.416214+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'a4628d13-c4e1-4719-a55a-252c995eec8a'),
			(9334, 'GitBlamePopupViewed', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go?L48:2', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 13:57:31.521936+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '6b444e53-72f3-4f0d-801b-24d3901d4de1'),
			(9355, 'SearchExportPerformed', 'https://sourcegraph.test:3443/search?q=context:global+test&patternType=standard', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{"count": 2}', '0.0.0+dev', '2022-09-05 14:01:07.795943+02', '{"code-ownership": true}', '2022-02-21', '{"count": 2}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '45061129-cae4-4a54-affe-b6a117eba412'),
			(9470, 'GitBlamePopupViewed', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:11:04.116582+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'a59dcf76-2566-43da-9d26-edf8c17cb4a4'),
			(9465, 'GitBlameEnabled', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:11:01.359624+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '990f9c3b-825d-40da-96d1-3cd482c777f3'),
			(9464, 'GitBlameDisabled', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:10:58.142905+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '457fb38e-3c70-4a81-93c2-cc389cda7f0d'),
			(9479, 'GitBlamePopupClicked', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:11:09.844475+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '2a64e895-c5d8-46fb-8dd4-9b231e9f00be'),
			(9503, 'GitBlamePopupClicked', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:12:28.32205+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '28f30f97-ee6b-4316-b5c9-8b8eb4d3867b'),
			(9504, 'GitBlamePopupViewed', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:12:28.32205+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '559b8f50-3042-4b75-9cf7-c9d03625c818'),
			(9462, 'GitBlameEnabled', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:10:54.295176+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'a1590e45-1186-4dc3-9967-388fb7018607'),
			(9463, 'GitBlamePopupViewed', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:10:57.117987+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '2ca8c2bd-3443-4fa2-ad7b-1737ea9e95b3'),
			(9468, 'GitBlamePopupViewed', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:11:03.12202+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '214e2a7d-6d1b-4dce-984a-0a56e1c8aacc'),
			(9469, 'GitBlamePopupViewed', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:11:03.122022+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'ad69b5ec-4db5-40b5-b545-423f8940855e'),
			(9471, 'GitBlamePopupViewed', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:11:05.129754+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'e9b982ae-3712-4da1-bc01-9c633cabf3c9'),
			(9488, 'SearchExportPerformed', 'https://sourcegraph.test:3443/search?q=context:global+test&patternType=standard', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{"count": 1}', '0.0.0+dev', '2022-09-05 14:11:26.163539+02', '{"code-ownership": true}', '2022-02-21', '{"count": 1}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '25187d54-9b6c-466b-a75a-d06bd0dde4ae'),
			(9495, 'SearchExportPerformed', 'https://sourcegraph.test:3443/search?q=context:global+test+test&patternType=standard', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{"count": 13}', '0.0.0+dev', '2022-09-05 14:11:57.159622+02', '{"code-ownership": true}', '2022-02-21', '{"count": 13}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'a4bebfb5-d251-4171-8fbb-9c4c2e79b148'),
			(9496, 'GitBlamePopupViewed', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:12:10.122718+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'bd241787-52e3-4517-b1a4-ad5e6477a12d'),
			(9501, 'GitBlamePopupViewed', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:12:24.300455+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '8cfc4a17-5b6c-4fc7-9f28-09ff14109859'),
			(9498, 'GitBlameEnabled', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:12:23.031747+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'de065e3b-d786-42e9-8a56-18304eda43a6'),
			(9502, 'GitBlamePopupViewed', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:12:26.290946+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '416efd98-c400-4f88-bf0c-27cf751c8a5c'),
			(9506, 'GitBlameDisabled', 'https://sourcegraph.test:3443/github.com/grafana/grafana/-/blob/pkg/setting/setting.go', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{}', '0.0.0+dev', '2022-09-05 14:12:32.084328+02', '{"code-ownership": true}', '2022-02-21', '{}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '4b7dc000-12db-4689-9471-ec41bbad16a7'),
			(9508, 'SearchExportPerformed', 'https://sourcegraph.test:3443/search?q=context:global+test+test&patternType=standard', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{"count": 13}', '0.0.0+dev', '2022-09-05 14:12:37.137032+02', '{"code-ownership": true}', '2022-02-21', '{"count": 13}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'fa316362-3c33-4bcd-a0d3-f35cdc5ec714'),
			(10521, 'OpenInEditorClicked', 'https://sourcegraph.test:3443/github.com/sourcegraph/sourcegraph/-/blob/dev/sg/os.go?L26', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{"editor": "vscode"}', '0.0.0+dev', '2022-09-07 13:14:20.641057+02', '{"code-ownership": true}', '2022-02-21', '{"editor": "vscode"}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '0eb2c3d9-f2c1-4e32-b68f-af47a58ac699'),
			(10529, 'OpenInEditorClicked', 'https://sourcegraph.test:3443/github.com/sourcegraph/sourcegraph/-/blob/dev/sg/os.go?L26', 1, 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', 'WEB', '{"editor": "goland"}', '0.0.0+dev', '2022-09-07 13:14:20.641057+02', '{"code-ownership": true}', '2022-02-21', '{"editor": "goland"}', 'https://sourcegraph.test:3443/search', 'https://sourcegraph.test:3443/search', '', 'a58d0dbd-46aa-4277-b0b4-883c5c6a886c', '0eb2c3d9-f2c1-4e32-b68f-af47a58ac699')
			`)
	if err != nil {
		t.Fatal(err)
	}

	have, err := GetMigratedExtensionsUsageStatistics(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	want := &types.MigratedExtensionsUsageStatistics{
		GitBlameEnabled:                 ptr(int32(6)),
		GitBlameEnabledUniqueUsers:      ptr(int32(1)),
		GitBlameDisabled:                ptr(int32(3)),
		GitBlameDisabledUniqueUsers:     ptr(int32(1)),
		GitBlamePopupViewed:             ptr(int32(13)),
		GitBlamePopupViewedUniqueUsers:  ptr(int32(1)),
		GitBlamePopupClicked:            ptr(int32(4)),
		GitBlamePopupClickedUniqueUsers: ptr(int32(1)),

		SearchExportPerformed:            ptr(int32(4)),
		SearchExportPerformedUniqueUsers: ptr(int32(1)),
		SearchExportFailed:               nil,
		SearchExportFailedUniqueUsers:    nil,

		OpenInEditor: []*types.MigratedExtensionsOpenInEditorUsageStatistics{
			{
				IdeKind:            "goland",
				Clicked:            ptr(int32(1)),
				ClickedUniqueUsers: ptr(int32(1)),
			},
			{
				IdeKind:            "vscode",
				Clicked:            ptr(int32(1)),
				ClickedUniqueUsers: ptr(int32(1)),
			},
		},
	}

	if diff := cmp.Diff(want, have); diff != "" {
		t.Fatal(diff)
	}
}
