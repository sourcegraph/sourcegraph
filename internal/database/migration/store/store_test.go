package store

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"golang.org/x/sync/errgroup"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/definition"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/shared"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestEnsureSchemaTable(t *testing.T) {
	db := dbtest.NewDB(t)
	store := testStore(db)
	ctx := context.Background()

	// Test initially missing table
	if err := store.Exec(ctx, sqlf.Sprintf("SELECT * FROM migration_logs")); err == nil {
		t.Fatalf("expected query to fail due to missing table migration_logs")
	}

	if err := store.EnsureSchemaTable(ctx); err != nil {
		t.Fatalf("unexpected error ensuring schema table exists: %s", err)
	}

	// Test table was created
	if err := store.Exec(ctx, sqlf.Sprintf("SELECT * FROM migration_logs")); err != nil {
		t.Fatalf("unexpected error querying migration_logs: %s", err)
	}

	// Test idempotencyl
	if err := store.EnsureSchemaTable(ctx); err != nil {
		t.Fatalf("expected method to be idempotent, got error: %s", err)
	}
}

func TestBackfillSchemaVersions(t *testing.T) {
	t.Run("frontend", func(t *testing.T) {
		t.Run("v4.1.0", func(t *testing.T) {
			appliedIDs := []int{1648051770, 1657107627, 1648195639, 1657279116, 1648524019, 1657279170, 1648628900, 1657635365, 1649159359, 1657663493, 1649253538, 1658122170, 1649269601, 1658174103, 1649432863, 1658225452, 1649441222, 1658255432, 1649759318, 1658384388, 1650456734, 1658484997, 1650637472, 1658503913, 1651061363, 1658512336, 1651077257, 1658748822, 1651159431, 1658837440, 1652143849, 1658856572, 1652175864, 1658874734, 1652189866, 1658950366, 1652228814, 1659085788, 1652707934, 1659368926, 1652946496, 1659380538, 1652964210, 1659434035, 1653334014, 1659459805, 1653472246, 1659721548, 1653479179, 1660132915, 1653524883, 1660312877, 1653596521, 1660710812, 1654116265, 1660710916, 1654168174, 1660711451, 1654770608, 1660742069, 1654848945, 1661441160, 1654872407, 1661502186, 1654874148, 1661507724, 1654874153, 1662467128, 1655037388, 1662636054, 1655037391, 1663569995, 1655067139, 1663665519, 1655105391, 1663871069, 1655128668, 1664300936, 1655157509, 1664988036, 1655226733, 1665056530, 1655328928, 1665138849, 1655412173, 1665399117, 1655454264, 1665420690, 1655481894, 1665488828, 1655737737, 1665524865, 1655763641, 1665588249, 1655843069, 1665646849, 1656447205, 1665770699, 1657106983}
			expectedIDs := []int{-1528395684, 1528395684, 1528395685, 1528395686, 1528395687, 1528395688, 1528395689, 1528395690, 1528395691, 1528395692, 1528395693, 1528395694, 1528395695, 1528395696, 1528395697, 1528395698, 1528395699, 1528395700, 1528395701, 1528395702, 1528395703, 1528395704, 1528395705, 1528395706, 1528395707, 1528395708, 1528395709, 1528395710, 1528395711, 1528395712, 1528395713, 1528395714, 1528395715, 1528395716, 1528395717, 1528395718, 1528395719, 1528395720, 1528395721, 1528395722, 1528395723, 1528395724, 1528395725, 1528395726, 1528395727, 1528395728, 1528395729, 1528395730, 1528395731, 1528395732, 1528395733, 1528395734, 1528395735, 1528395736, 1528395737, 1528395738, 1528395739, 1528395740, 1528395741, 1528395742, 1528395743, 1528395744, 1528395745, 1528395746, 1528395747, 1528395748, 1528395749, 1528395750, 1528395751, 1528395752, 1528395753, 1528395754, 1528395755, 1528395756, 1528395757, 1528395758, 1528395759, 1528395760, 1528395761, 1528395762, 1528395763, 1528395764, 1528395765, 1528395766, 1528395767, 1528395768, 1528395769, 1528395770, 1528395771, 1528395772, 1528395773, 1528395774, 1528395775, 1528395776, 1528395777, 1528395778, 1528395779, 1528395780, 1528395781, 1528395782, 1528395783, 1528395784, 1528395785, 1528395786, 1528395787, 1528395788, 1528395789, 1528395790, 1528395791, 1528395792, 1528395793, 1528395794, 1528395795, 1528395796, 1528395797, 1528395798, 1528395799, 1528395800, 1528395801, 1528395802, 1528395803, 1528395804, 1528395805, 1528395806, 1528395807, 1528395808, 1528395809, 1528395810, 1528395811, 1528395812, 1528395813, 1528395814, 1528395815, 1528395816, 1528395817, 1528395818, 1528395819, 1528395820, 1528395821, 1528395822, 1528395823, 1528395824, 1528395825, 1528395826, 1528395827, 1528395828, 1528395829, 1528395830, 1528395831, 1528395832, 1528395833, 1528395834, 1528395835, 1528395836, 1528395837, 1528395838, 1528395839, 1528395840, 1528395841, 1528395842, 1528395843, 1528395844, 1528395845, 1528395846, 1528395847, 1528395848, 1528395849, 1528395850, 1528395851, 1528395852, 1528395853, 1528395854, 1528395855, 1528395856, 1528395857, 1528395858, 1528395859, 1528395860, 1528395861, 1528395862, 1528395863, 1528395864, 1528395865, 1528395866, 1528395867, 1528395868, 1528395869, 1528395870, 1528395871, 1528395872, 1528395873, 1528395874, 1528395875, 1528395876, 1528395877, 1528395878, 1528395879, 1528395880, 1528395881, 1528395882, 1528395883, 1528395884, 1528395885, 1528395886, 1528395887, 1528395888, 1528395889, 1528395890, 1528395891, 1528395892, 1528395893, 1528395894, 1528395895, 1528395896, 1528395897, 1528395898, 1528395899, 1528395900, 1528395901, 1528395902, 1528395903, 1528395904, 1528395905, 1528395906, 1528395907, 1528395908, 1528395909, 1528395910, 1528395911, 1528395912, 1528395913, 1528395914, 1528395915, 1528395916, 1528395917, 1528395918, 1528395919, 1528395920, 1528395921, 1528395922, 1528395923, 1528395924, 1528395925, 1528395926, 1528395927, 1528395928, 1528395929, 1528395930, 1528395931, 1528395932, 1528395933, 1528395934, 1528395935, 1528395936, 1528395937, 1528395938, 1528395939, 1528395940, 1528395941, 1528395942, 1528395943, 1528395944, 1528395945, 1528395946, 1528395947, 1528395948, 1528395949, 1528395950, 1528395951, 1528395952, 1528395953, 1528395954, 1528395955, 1528395956, 1528395957, 1528395958, 1528395959, 1528395960, 1528395961, 1528395962, 1528395963, 1528395964, 1528395965, 1528395966, 1528395967, 1528395968, 1528395969, 1528395970, 1528395971, 1528395972, 1528395973, 1644471839, 1644515056, 1644583379, 1644868458, 1645106226, 1645554732, 1645635177, 1646027072, 1646153853, 1646239940, 1646306565, 1646652951, 1646741362, 1646847163, 1646848295, 1647282553, 1647849753, 1647860082}
			testViaMigrationLogs(t, "frontend", appliedIDs, append(expectedIDs, appliedIDs...))
		})

		t.Run("v5.0.2", func(t *testing.T) {
			appliedIDs := []int{1648051770, 1648195639, 1648524019, 1648628900, 1649159359, 1649253538, 1649269601, 1649432863, 1649441222, 1649759318, 1650456734, 1650637472, 1651061363, 1651077257, 1651159431, 1652143849, 1652175864, 1652189866, 1652228814, 1652707934, 1652946496, 1652964210, 1653334014, 1653472246, 1653479179, 1653524883, 1653596521, 1654116265, 1654168174, 1654770608, 1654848945, 1654872407, 1654874148, 1654874153, 1655037388, 1655037391, 1655067139, 1655105391, 1655128668, 1655157509, 1655226733, 1655328928, 1655412173, 1655454264, 1655481894, 1655737737, 1655763641, 1655843069, 1656447205, 1657106983, 1657107627, 1657279116, 1657279170, 1657635365, 1657663493, 1658122170, 1658174103, 1658225452, 1658255432, 1658384388, 1658484997, 1658503913, 1658512336, 1658748822, 1658837440, 1658856572, 1658874734, 1658950366, 1659085788, 1659368926, 1659380538, 1659434035, 1659459805, 1659721548, 1660132915, 1660312877, 1660710812, 1660710916, 1660711451, 1660742069, 1661441160, 1661502186, 1661507724, 1662467128, 1662636054, 1663569995, 1663665519, 1663871069, 1664300936, 1664897165, 1664988036, 1665056530, 1665138849, 1665399117, 1665420690, 1665477911, 1665488828, 1665524865, 1665588249, 1665646849, 1665770699, 1666034720, 1666131819, 1666145729, 1666344635, 1666398757, 1666524436, 1666598814, 1666598828, 1666598983, 1666598987, 1666598990, 1666717223, 1666886757, 1666904087, 1666939263, 1667220502, 1667220626, 1667220628, 1667220768, 1667222952, 1667259203, 1667313173, 1667395984, 1667433265, 1667497565, 1667500111, 1667825028, 1667848448, 1667863757, 1667917030, 1667950421, 1667952974, 1668174127, 1668179496, 1668179619, 1668184279, 1668603582, 1668707631, 1668767882, 1668808118, 1668813365, 1669184869, 1669297489, 1669576792, 1669645608, 1669836151, 1670256530, 1670350006, 1670539388, 1670539913, 1670542168, 1670543231, 1670600028, 1670870072, 1670934184, 1671159453, 1671463799, 1671543381, 1672884222, 1672897105, 1673019611, 1673351808, 1673405886, 1673871310, 1673897709, 1674035302, 1674041632, 1674047296, 1674455760, 1674480050, 1674642349, 1674669326, 1674669794, 1674754280, 1674814035, 1674952295, 1675155867, 1675194688, 1675257827, 1675277218, 1675277500, 1675277968, 1675296942, 1675367314, 1675647612, 1675850599, 1675864432, 1675962678, 1676272751, 1676328864, 1676420496, 1676584791, 1676996650, 1677003167, 1677005673, 1677008591, 1677073533, 1677104938, 1677166643, 1677242688, 1677483453, 1677594756, 1677607213, 1677627515, 1677627559, 1677627566, 1677694168, 1677694170, 1677700103, 1677716184, 1677716470, 1677803354, 1677811663, 1677878270, 1677944569, 1677944752, 1677945580, 1677955553, 1677958359, 1678091683, 1678112318, 1678175532, 1678213774, 1678214530, 1678220614, 1678290792, 1678291091, 1678291402, 1678291831, 1678320579, 1678380933, 1678409821, 1678456448, 1678601228, 1678832491, 1678899992, 1678994673, 1680707560}
			expectedIDs := []int{-1528395684, 1528395684, 1528395685, 1528395686, 1528395687, 1528395688, 1528395689, 1528395690, 1528395691, 1528395692, 1528395693, 1528395694, 1528395695, 1528395696, 1528395697, 1528395698, 1528395699, 1528395700, 1528395701, 1528395702, 1528395703, 1528395704, 1528395705, 1528395706, 1528395707, 1528395708, 1528395709, 1528395710, 1528395711, 1528395712, 1528395713, 1528395714, 1528395715, 1528395716, 1528395717, 1528395718, 1528395719, 1528395720, 1528395721, 1528395722, 1528395723, 1528395724, 1528395725, 1528395726, 1528395727, 1528395728, 1528395729, 1528395730, 1528395731, 1528395732, 1528395733, 1528395734, 1528395735, 1528395736, 1528395737, 1528395738, 1528395739, 1528395740, 1528395741, 1528395742, 1528395743, 1528395744, 1528395745, 1528395746, 1528395747, 1528395748, 1528395749, 1528395750, 1528395751, 1528395752, 1528395753, 1528395754, 1528395755, 1528395756, 1528395757, 1528395758, 1528395759, 1528395760, 1528395761, 1528395762, 1528395763, 1528395764, 1528395765, 1528395766, 1528395767, 1528395768, 1528395769, 1528395770, 1528395771, 1528395772, 1528395773, 1528395774, 1528395775, 1528395776, 1528395777, 1528395778, 1528395779, 1528395780, 1528395781, 1528395782, 1528395783, 1528395784, 1528395785, 1528395786, 1528395787, 1528395788, 1528395789, 1528395790, 1528395791, 1528395792, 1528395793, 1528395794, 1528395795, 1528395796, 1528395797, 1528395798, 1528395799, 1528395800, 1528395801, 1528395802, 1528395803, 1528395804, 1528395805, 1528395806, 1528395807, 1528395808, 1528395809, 1528395810, 1528395811, 1528395812, 1528395813, 1528395814, 1528395815, 1528395816, 1528395817, 1528395818, 1528395819, 1528395820, 1528395821, 1528395822, 1528395823, 1528395824, 1528395825, 1528395826, 1528395827, 1528395828, 1528395829, 1528395830, 1528395831, 1528395832, 1528395833, 1528395834, 1528395835, 1528395836, 1528395837, 1528395838, 1528395839, 1528395840, 1528395841, 1528395842, 1528395843, 1528395844, 1528395845, 1528395846, 1528395847, 1528395848, 1528395849, 1528395850, 1528395851, 1528395852, 1528395853, 1528395854, 1528395855, 1528395856, 1528395857, 1528395858, 1528395859, 1528395860, 1528395861, 1528395862, 1528395863, 1528395864, 1528395865, 1528395866, 1528395867, 1528395868, 1528395869, 1528395870, 1528395871, 1528395872, 1528395873, 1528395874, 1528395875, 1528395876, 1528395877, 1528395878, 1528395879, 1528395880, 1528395881, 1528395882, 1528395883, 1528395884, 1528395885, 1528395886, 1528395887, 1528395888, 1528395889, 1528395890, 1528395891, 1528395892, 1528395893, 1528395894, 1528395895, 1528395896, 1528395897, 1528395898, 1528395899, 1528395900, 1528395901, 1528395902, 1528395903, 1528395904, 1528395905, 1528395906, 1528395907, 1528395908, 1528395909, 1528395910, 1528395911, 1528395912, 1528395913, 1528395914, 1528395915, 1528395916, 1528395917, 1528395918, 1528395919, 1528395920, 1528395921, 1528395922, 1528395923, 1528395924, 1528395925, 1528395926, 1528395927, 1528395928, 1528395929, 1528395930, 1528395931, 1528395932, 1528395933, 1528395934, 1528395935, 1528395936, 1528395937, 1528395938, 1528395939, 1528395940, 1528395941, 1528395942, 1528395943, 1528395944, 1528395945, 1528395946, 1528395947, 1528395948, 1528395949, 1528395950, 1528395951, 1528395952, 1528395953, 1528395954, 1528395955, 1528395956, 1528395957, 1528395958, 1528395959, 1528395960, 1528395961, 1528395962, 1528395963, 1528395964, 1528395965, 1528395966, 1528395967, 1528395968, 1528395969, 1528395970, 1528395971, 1528395972, 1528395973, 1644471839, 1644515056, 1644583379, 1644868458, 1645106226, 1645554732, 1645635177, 1646027072, 1646153853, 1646239940, 1646306565, 1646652951, 1646741362, 1646847163, 1646848295, 1647282553, 1647849753, 1647860082}
			testViaMigrationLogs(t, "frontend", appliedIDs, append(expectedIDs, appliedIDs...))
		})
	})
}

// testViaMigrationLogs asserts the given expected versions are backfilled on a new store instance, given
// the migration_logs table has an entry with the given initial version.
func testViaMigrationLogs(t *testing.T, schemaName string, initialVersions []int, expectedVersions []int) {
	testBackfillSchemaVersion(t, schemaName, expectedVersions, func(ctx context.Context, store *Store) {
		if err := setupMigrationLogsTest(ctx, store, schemaName, initialVersions); err != nil {
			t.Fatalf("unexpected error preparing migration_logs tests: %s", err)
		}
	})
}

// setupMigrationLogsTest populates the migration_logs table with the given versions.
func setupMigrationLogsTest(ctx context.Context, store *Store, schemaName string, versions []int) error {
	for _, version := range versions {
		if err := store.Exec(ctx, sqlf.Sprintf(`
		INSERT INTO migration_logs (
			migration_logs_schema_version,
			schema,
			version,
			up,
			started_at,
			finished_at,
			success
		) VALUES (%s, %s, %s, true, NOW(), NOW(), true)
	`,
			currentMigrationLogSchemaVersion,
			schemaName,
			version,
		)); err != nil {
			return err
		}
	}

	return nil
}

// testBackfillSchemaVersion runs the given setup function prior to backfilling a test
// migration store. The versions available post-backfill are checked against the given
// expected versions.
func testBackfillSchemaVersion(
	t *testing.T,
	schemaName string,
	expectedVersions []int,
	setup func(ctx context.Context, store *Store),
) {
	db := dbtest.NewDB(t)
	store := testStoreWithName(db, schemaName)
	ctx := context.Background()

	if err := store.EnsureSchemaTable(ctx); err != nil {
		t.Fatalf("unexpected error ensuring schema table exists: %s", err)
	}

	setup(ctx, store)

	if err := store.BackfillSchemaVersions(ctx); err != nil {
		t.Fatalf("unexpected error backfilling schema table: %s", err)
	}

	appliedVersions, _, _, err := store.Versions(ctx)
	if err != nil {
		t.Fatalf("unexpected error querying versions: %s", err)
	}

	sort.Ints(appliedVersions)
	for _, version := range backfillOverrides {
		expectedVersions = append(expectedVersions, int(version))
	}
	sort.Ints(expectedVersions)
	if diff := cmp.Diff(expectedVersions, appliedVersions); diff != "" {
		t.Errorf("unexpected applied migrations (-want +got):\n%s", diff)
	}
}

func TestHumanizeSchemaName(t *testing.T) {
	for input, expected := range map[string]string{
		"schema_migrations":              "frontend",
		"codeintel_schema_migrations":    "codeintel",
		"codeinsights_schema_migrations": "codeinsights",
		"test_schema_migrations":         "test",
	} {
		if output := humanizeSchemaName(input); output != expected {
			t.Errorf("unexpected output. want=%q have=%q", expected, output)
		}
	}
}

func TestVersions(t *testing.T) {
	db := dbtest.NewDB(t)
	store := testStore(db)
	ctx := context.Background()
	if err := store.EnsureSchemaTable(ctx); err != nil {
		t.Fatalf("unexpected error ensuring schema table exists: %s", err)
	}

	t.Run("empty", func(*testing.T) {
		if appliedVersions, pendingVersions, failedVersions, err := store.Versions(ctx); err != nil {
			t.Fatalf("unexpected error querying versions: %s", err)
		} else if len(appliedVersions)+len(pendingVersions)+len(failedVersions) > 0 {
			t.Fatalf("unexpected no versions, got applied=%v pending=%v failed=%v", appliedVersions, pendingVersions, failedVersions)
		}
	})

	type testCase struct {
		startedAt    time.Time
		version      int
		up           bool
		success      *bool
		errorMessage *string
	}
	makeCase := func(t time.Time, version int, up bool, failed *bool) testCase {
		if failed == nil {
			return testCase{t, version, up, nil, nil}
		}
		if *failed {
			return testCase{t, version, up, pointers.Ptr(false), pointers.Ptr("uh-oh")}
		}
		return testCase{t, version, up, pointers.Ptr(true), nil}
	}

	t3 := timeutil.Now()
	t2 := t3.Add(-time.Hour * 24)
	t1 := t2.Add(-time.Hour * 24)

	for _, migrationLog := range []testCase{
		// Historic attempts
		makeCase(t1, 1003, true, pointers.Ptr(true)), makeCase(t2, 1003, false, pointers.Ptr(true)), // 1003: successful up, successful down
		makeCase(t1, 1004, true, pointers.Ptr(true)),                                                // 1004: successful up
		makeCase(t1, 1006, true, pointers.Ptr(false)), makeCase(t2, 1006, true, pointers.Ptr(true)), // 1006: failed up, successful up

		// Last attempts
		makeCase(t3, 1001, true, pointers.Ptr(false)),  // successful up
		makeCase(t3, 1002, false, pointers.Ptr(false)), // successful down
		makeCase(t3, 1003, true, nil),                  // pending up
		makeCase(t3, 1004, false, nil),                 // pending down
		makeCase(t3, 1005, true, pointers.Ptr(true)),   // failed up
		makeCase(t3, 1006, false, pointers.Ptr(true)),  // failed down
	} {
		finishedAt := &migrationLog.startedAt
		if migrationLog.success == nil {
			finishedAt = nil
		}

		if err := store.Exec(ctx, sqlf.Sprintf(`INSERT INTO migration_logs (
				migration_logs_schema_version,
				schema,
				version,
				up,
				started_at,
				success,
				finished_at,
				error_message
			) VALUES (%s, %s, %s, %s, %s, %s, %s, %s)`,
			currentMigrationLogSchemaVersion,
			defaultTestTableName,
			migrationLog.version,
			migrationLog.up,
			migrationLog.startedAt,
			migrationLog.success,
			finishedAt,
			migrationLog.errorMessage,
		)); err != nil {
			t.Fatalf("unexpected error inserting data: %s", err)
		}
	}

	assertVersions(
		t,
		ctx,
		store,
		[]int{1001},       // expectedAppliedVersions
		[]int{1003, 1004}, // expectedPendingVersions
		[]int{1005, 1006}, // expectedFailedVersions
	)
}

func TestTryLock(t *testing.T) {
	db := dbtest.NewDB(t)
	store := testStore(db)
	ctx := context.Background()

	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("failed to open new connection: %s", err)
	}
	t.Cleanup(func() { conn.Close() })

	// Acquire lock in distinct session
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1, 0)`, store.lockKey()); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// TryLock should fail
	if acquired, _, err := store.TryLock(ctx); err != nil {
		t.Fatalf("unexpected error acquiring lock: %s", err)
	} else if acquired {
		t.Fatalf("expected lock to be held by another session")
	}

	// Drop lock
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1, 0)`, store.lockKey()); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// TryLock should succeed
	acquired, unlock, err := store.TryLock(ctx)
	if err != nil {
		t.Fatalf("unexpected error acquiring lock: %s", err)
	} else if !acquired {
		t.Fatalf("expected lock to be acquired")
	}

	if err := unlock(nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	// Check idempotency
	if err := unlock(nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestWrappedUp(t *testing.T) {
	db := dbtest.NewDB(t)
	store := testStore(db)
	ctx := context.Background()

	if err := store.EnsureSchemaTable(ctx); err != nil {
		t.Fatalf("unexpected error ensuring schema table exists: %s", err)
	}

	// Seed a few migrations
	for _, id := range []int{13, 14, 15} {
		def := definition.Definition{
			ID:      id,
			UpQuery: sqlf.Sprintf(`-- No-op`),
		}
		f := func() error {
			return store.Up(ctx, def)
		}
		if err := store.WithMigrationLog(ctx, def, true, f); err != nil {
			t.Fatalf("unexpected error running migration: %s", err)
		}
	}

	logs := []migrationLog{
		{
			Schema:  defaultTestTableName,
			Version: 13,
			Up:      true,
			Success: pointers.Ptr(true),
		},
		{
			Schema:  defaultTestTableName,
			Version: 14,
			Up:      true,
			Success: pointers.Ptr(true),
		},
		{
			Schema:  defaultTestTableName,
			Version: 15,
			Up:      true,
			Success: pointers.Ptr(true),
		},
	}

	t.Run("success", func(t *testing.T) {
		def := definition.Definition{
			ID: 16,
			UpQuery: sqlf.Sprintf(`
				CREATE TABLE test_trees (
					name text,
					leaf_type text,
					seed_type text,
					bark_type text
				);
				INSERT INTO test_trees VALUES
					('oak', 'broad', 'regular', 'strong'),
					('birch', 'narrow', 'regular', 'flaky'),
					('pine', 'needle', 'pine cone', 'soft');
			`),
		}
		f := func() error {
			return store.Up(ctx, def)
		}
		if err := store.WithMigrationLog(ctx, def, true, f); err != nil {
			t.Fatalf("unexpected error running migration: %s", err)
		}

		if barkType, _, err := basestore.ScanFirstString(store.Query(ctx, sqlf.Sprintf(`SELECT bark_type FROM test_trees WHERE name = 'birch'`))); err != nil {
			t.Fatalf("migration query did not succeed; unexpected error querying test table: %s", err)
		} else if barkType != "flaky" {
			t.Fatalf("migration query did not succeed; unexpected bark type. want=%s have=%s", "flaky", barkType)
		}

		logs = append(logs, migrationLog{
			Schema:  defaultTestTableName,
			Version: 16,
			Up:      true,
			Success: pointers.Ptr(true),
		})
		assertLogs(t, ctx, store, logs)
		assertVersions(t, ctx, store, []int{13, 14, 15, 16}, nil, nil)
	})

	t.Run("query failure", func(t *testing.T) {
		expectedErrorMessage := "ERROR: relation"

		def := definition.Definition{
			ID: 17,
			UpQuery: sqlf.Sprintf(`
				-- Note: table already exists
				CREATE TABLE test_trees (
					name text,
					leaf_type text,
					seed_type text,
					bark_type text
				);
			`),
		}
		f := func() error {
			return store.Up(ctx, def)
		}
		if err := store.WithMigrationLog(ctx, def, true, f); err == nil || !strings.Contains(err.Error(), expectedErrorMessage) {
			t.Fatalf("unexpected error want=%q have=%q", expectedErrorMessage, err)
		}

		logs = append(logs, migrationLog{
			Schema:  defaultTestTableName,
			Version: 17,
			Up:      true,
			Success: pointers.Ptr(false),
		})
		assertLogs(t, ctx, store, logs)
		assertVersions(t, ctx, store, []int{13, 14, 15, 16}, nil, []int{17})
	})
}

func TestWrappedDown(t *testing.T) {
	db := dbtest.NewDB(t)
	store := testStore(db)
	ctx := context.Background()

	if err := store.EnsureSchemaTable(ctx); err != nil {
		t.Fatalf("unexpected error ensuring schema table exists: %s", err)
	}

	if err := store.Exec(ctx, sqlf.Sprintf(`
		CREATE TABLE test_trees (
			name text,
			leaf_type text,
			seed_type text,
			bark_type text
		);
	`)); err != nil {
		t.Fatalf("unexpected error creating test table: %s", err)
	}

	testQuery := sqlf.Sprintf(`
		INSERT INTO test_trees VALUES
			('oak', 'broad', 'regular', 'strong'),
			('birch', 'narrow', 'regular', 'flaky'),
			('pine', 'needle', 'pine cone', 'soft');
	`)

	// run twice to ensure the error post-migration is not due to an index constraint
	if err := store.Exec(ctx, testQuery); err != nil {
		t.Fatalf("unexpected error inserting into test table: %s", err)
	}
	if err := store.Exec(ctx, testQuery); err != nil {
		t.Fatalf("unexpected error inserting into test table: %s", err)
	}

	// Seed a few migrations
	for _, id := range []int{12, 13, 14} {
		def := definition.Definition{
			ID:      id,
			UpQuery: sqlf.Sprintf(`-- No-op`),
		}
		f := func() error {
			return store.Up(ctx, def)
		}
		if err := store.WithMigrationLog(ctx, def, true, f); err != nil {
			t.Fatalf("unexpected error running migration: %s", err)
		}
	}

	logs := []migrationLog{
		{
			Schema:  defaultTestTableName,
			Version: 12,
			Up:      true,
			Success: pointers.Ptr(true),
		},
		{
			Schema:  defaultTestTableName,
			Version: 13,
			Up:      true,
			Success: pointers.Ptr(true),
		},
		{
			Schema:  defaultTestTableName,
			Version: 14,
			Up:      true,
			Success: pointers.Ptr(true),
		},
	}

	t.Run("success", func(t *testing.T) {
		def := definition.Definition{
			ID: 14,
			DownQuery: sqlf.Sprintf(`
				DROP TABLE test_trees;
			`),
		}
		f := func() error {
			return store.Down(ctx, def)
		}
		if err := store.WithMigrationLog(ctx, def, false, f); err != nil {
			t.Fatalf("unexpected error running migration: %s", err)
		}

		// note: this query succeeded twice earlier
		if err := store.Exec(ctx, testQuery); err == nil || !strings.Contains(err.Error(), "SQL Error") {
			t.Fatalf("migration query did not succeed; expected missing table. want=%q have=%q", "SQL Error", err)
		}

		logs = append(logs, migrationLog{
			Schema:  defaultTestTableName,
			Version: 14,
			Up:      false,
			Success: pointers.Ptr(true),
		})
		assertLogs(t, ctx, store, logs)
		assertVersions(t, ctx, store, []int{12, 13}, nil, nil)
	})

	t.Run("query failure", func(t *testing.T) {
		expectedErrorMessage := "ERROR: syntax error at or near"

		def := definition.Definition{
			ID: 13,
			DownQuery: sqlf.Sprintf(`
				-- Note: table does not exist
				DROP TABLE TABLE test_trees;
			`),
		}
		f := func() error {
			return store.Down(ctx, def)
		}
		if err := store.WithMigrationLog(ctx, def, false, f); err == nil || !strings.Contains(err.Error(), expectedErrorMessage) {
			t.Fatalf("unexpected error want=%q have=%q", expectedErrorMessage, err)
		}

		logs = append(logs, migrationLog{
			Schema:  defaultTestTableName,
			Version: 13,
			Up:      false,
			Success: pointers.Ptr(false),
		})
		assertLogs(t, ctx, store, logs)
		assertVersions(t, ctx, store, []int{12, 13}, nil, nil)
	})
}

func TestIndexStatus(t *testing.T) {
	db := dbtest.NewDB(t)
	store := testStore(db)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, "CREATE TABLE tbl (id text, name text);"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Index does not (yet) exist
	if _, ok, err := store.IndexStatus(ctx, "tbl", "idx"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	} else if ok {
		t.Fatalf("unexpected index status")
	}

	// Wrap context in a small timeout; we do tight for-loops here to determine
	// when we can continue on to/unblock the next operation, but none of the
	// steps should take any significant time.
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	group, groupCtx := errgroup.WithContext(ctx)
	defer cancel()

	whileEmpty := func(ctx context.Context, conn dbutil.DB, query string) error {
		for {
			rows, err := conn.QueryContext(ctx, query)
			if err != nil {
				return err
			}

			lockVisible := rows.Next()

			if err := basestore.CloseRows(rows, nil); err != nil {
				return err
			}

			if lockVisible {
				return nil
			}
		}
	}

	// Create separate connections to precise control contention of resources
	// so we can examine what this method returns while an index is being created.

	conns := make([]*sql.Conn, 3)
	for i := range 3 {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("failed to open new connection: %s", err)
		}
		t.Cleanup(func() { conn.Close() })

		conns[i] = conn
	}
	connA, connB, connC := conns[0], conns[1], conns[2]

	lockQuery := `SELECT pg_advisory_lock(10, 10)`
	unlockQuery := `SELECT pg_advisory_unlock(10, 10)`
	createIndexQuery := `CREATE INDEX CONCURRENTLY idx ON tbl(id)`

	// Session A
	// Successfully take and hold advisory lock
	if _, err := connA.ExecContext(ctx, lockQuery); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Session B
	// Try to take advisory lock; blocked by Session A
	group.Go(func() error {
		_, err := connB.ExecContext(groupCtx, lockQuery)
		return err
	})

	// Session C
	// try to create index concurrently; blocked by session B waiting on session A
	group.Go(func() error {
		// Wait until we can see Session B's lock before attempting to create index
		if err := whileEmpty(groupCtx, connC, "SELECT 1 FROM pg_locks WHERE locktype = 'advisory' AND NOT granted"); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		_, err := connC.ExecContext(groupCtx, createIndexQuery)
		return err
	})

	// "waiting for old snapshots" will be the phase that is blocked by the concurrent
	// sessions holding advisory locks. We may happen to hit one of the earlier phases
	// if we're quick enough, so we'll keep polling progress until we hit the target.
	blockingPhase := "waiting for old snapshots"
	nonblockingPhasePrefixes := make([]string, 0, len(shared.CreateIndexConcurrentlyPhases))
	for _, prefix := range shared.CreateIndexConcurrentlyPhases {
		if prefix == blockingPhase {
			break
		}

		nonblockingPhasePrefixes = append(nonblockingPhasePrefixes, prefix)
	}
	compareWithPrefix := func(value, prefix string) bool {
		return value == prefix || strings.HasPrefix(value, prefix+":")
	}

	start := time.Now()
	const missingIndexThreshold = time.Second * 10

retryLoop:
	for {
		if status, ok, err := store.IndexStatus(ctx, "tbl", "idx"); err != nil {
			t.Fatalf("unexpected error: %s", err)
		} else if !ok {
			// Give a small amount of time for Session C to begin creating the index. Signaling
			// when Postgres has started to create the index is as difficult and expensive as
			// querying the index the status, so we just poll here for a relatively short time.
			if time.Since(start) >= missingIndexThreshold {
				t.Fatalf("expected index status after %s", missingIndexThreshold)
			}
		} else if status.Phase == nil {
			t.Fatalf("unexpected phase. want=%q have=nil", blockingPhase)
		} else if *status.Phase == blockingPhase {
			break
		} else {
			for _, prefix := range nonblockingPhasePrefixes {
				if compareWithPrefix(*status.Phase, prefix) {
					continue retryLoop
				}
			}

			t.Fatalf("unexpected phase. want=%q have=%q", blockingPhase, *status.Phase)
		}
	}

	// Session A
	// Unlock, unblocking both Session B and Session C
	if _, err := connA.ExecContext(ctx, unlockQuery); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Wait for index creation to complete
	if err := group.Wait(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if status, ok, err := store.IndexStatus(ctx, "tbl", "idx"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	} else if !ok {
		t.Fatalf("expected index status")
	} else {
		if !status.IsValid {
			t.Fatalf("unexpected isvalid. want=%v have=%v", true, status.IsValid)
		}
		if status.Phase != nil {
			t.Fatalf("unexpected phase. want=%v have=%v", nil, status.Phase)
		}
	}
}

const defaultTestTableName = "test_migrations_table"

func testStore(db *sql.DB) *Store {
	return testStoreWithName(db, defaultTestTableName)
}

func testStoreWithName(db *sql.DB, name string) *Store {
	return NewWithDB(&observation.TestContext, db, name)
}

func assertLogs(t *testing.T, ctx context.Context, store *Store, expectedLogs []migrationLog) {
	t.Helper()

	sort.Slice(expectedLogs, func(i, j int) bool {
		return expectedLogs[i].Version < expectedLogs[j].Version
	})

	logs, err := scanMigrationLogs(store.Query(ctx, sqlf.Sprintf(`SELECT schema, version, up, success FROM migration_logs ORDER BY version`)))
	if err != nil {
		t.Fatalf("unexpected error scanning logs: %s", err)
	}

	if diff := cmp.Diff(expectedLogs, logs); diff != "" {
		t.Errorf("unexpected migration logs (-want +got):\n%s", diff)
	}
}

func assertVersions(t *testing.T, ctx context.Context, store *Store, expectedAppliedVersions, expectedPendingVersions, expectedFailedVersions []int) {
	t.Helper()

	appliedVersions, pendingVersions, failedVersions, err := store.Versions(ctx)
	if err != nil {
		t.Fatalf("unexpected error querying version: %s", err)
	}

	if diff := cmp.Diff(expectedAppliedVersions, appliedVersions); diff != "" {
		t.Errorf("unexpected applied migration logs (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(expectedPendingVersions, pendingVersions); diff != "" {
		t.Errorf("unexpected pending migration logs (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(expectedFailedVersions, failedVersions); diff != "" {
		t.Errorf("unexpected failed migration logs (-want +got):\n%s", diff)
	}
}
