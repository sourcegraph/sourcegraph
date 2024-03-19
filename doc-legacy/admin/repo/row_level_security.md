# Row-level security

> NOTE: this document is deprecated, but preserved for historical and informational purposes.

Starting with version 9.5, Postgres provides a [row-level security](https://www.postgresql.org/docs/13/ddl-rowsecurity.html) mechanism (abbreviated as "RLS") that can restrict table access in a granular, per-user fashion. Sourcegraph uses this mechanism to provide data isolation and protection guarantees beyond those supplied by application-level techniques. This document serves as a brief overview of the concept, its application at Sourcegraph and administrative implications.

## Basics of RLS

Row-level security is enabled for a given table using the `ALTER TABLE <name> ENABLE ROW LEVEL SECURITY` statement. Once executed, all rows within that table immediately become inaccessible to all users _except_ for the table owner or superuser roles who have the `BYPASSRLS` attribute set. Access must then be explicitly permitted by the creation of one or more security policies which are then applied to the table.

Sourcegraph currently uses a single row security policy, which is applied to the `repo` table and covers all commands (`INSERT`, `SELECT`, etc.)

```
# select tablename, policyname, roles, cmd, format('%s...', left(qual, 16)) as policy from pg_policies;
┌───────────┬───────────────────────┬──────────────┬─────┬─────────────────────┐
│ tablename │      policyname       │    roles     │ cmd │       policy        │
╞═══════════╪═══════════════════════╪══════════════╪═════╪═════════════════════╡
│ repo      │ sg_repo_access_policy │ {sg_service} │ ALL │ (((NOT (current_... │
└───────────┴───────────────────────┴──────────────┴─────┴─────────────────────┘
(1 row)

Time: 0.657 ms
```

## Reducing privileges

It's not feasible to create a Postgres role for each individual Sourcegraph user. Instead, a dedicated `sg_service` role has been introduced that services can assume to downgrade their own capabilities on demand.

```
# select rolname, rolcanlogin, rolbypassrls from pg_roles where rolname like 'sg_%';
┌────────────┬─────────────┬──────────────┐
│  rolname   │ rolcanlogin │ rolbypassrls │
╞════════════╪═════════════╪══════════════╡
│ sg_service │ f           │ f            │
└────────────┴─────────────┴──────────────┘
(1 row)

Time: 24.462 ms
```

The `sg_service` role is not associated with any particular application-level Sourcegraph user, nor is it a user capable of logging in by itself. The policy applied to the `repo` table requires several `rls` values to be set, and these values dynamically alter how each query will behave.

For example, the default `sourcegraph` role in this sample database is permitted to see all 552 rows in the `repo` table because it's the owner of the table.

```
# select current_user;
┌──────────────┐
│ current_user │
╞══════════════╡
│ sourcegraph  │
└──────────────┘
(1 row)

Time: 0.197 ms

# select count(1) from repo;
┌───────┐
│ count │
╞═══════╡
│   552 │
└───────┘
(1 row)

Time: 15.781 ms
```

Once the `sg_service` role is assumed, Postgres needs additional information about which Sourcegraph user is executing the query. In this case, user 42 does not have permission to see the repositories owned by user 1 and no rows are returned. Note that we are executing the same query as before, but receiving different results.

```
# set role sg_service;
SET
Time: 1.187 ms

# set rls.user_id = 42;
SET
Time: 1.206 ms

# set rls.permission = 'read';
SET
Time: 0.333 ms

# set rls.use_permissions_user_mapping = true;
SET
Time: 0.327 ms

# select current_user;
┌──────────────┐
│ current_user │
╞══════════════╡
│ sg_service   │
└──────────────┘
(1 row)

Time: 0.381 ms

# select count(1) from repo;
┌───────┐
│ count │
╞═══════╡
│     0 │
└───────┘
(1 row)

Time: 28.288 ms
```

## Bypassing RLS

Row-level security can be bypassed by setting the `BYPASSRLS` attribute on a role. For example, if we were to create a `poweruser` role without this attribute, the existing security policy would prevent access to the `repo` table by default.

```
# create role poweruser;
CREATE ROLE
Time: 7.015 ms

# set role poweruser;
SET
Time: 0.349 ms

# select count(1) from repo;
┌───────┐
│ count │
╞═══════╡
│     0 │
└───────┘
(1 row)

Time: 21.373 ms
```

We can alter this role to set the `BYPASSRLS` attribute, at which point the security policy will be skipped and the role will have the normal level of access it would expect.

```
# alter role poweruser bypassrls;
ALTER ROLE
Time: 0.852 ms

# set role poweruser;
SET
Time: 0.229 ms

# select count(1) from repo;
┌───────┐
│ count │
╞═══════╡
│   552 │
└───────┘
(1 row)

Time: 6.280 ms
```

Additionally, it is possible to bypass RLS by supplying a policy that explicitly allows a particular role to access the table.

```
# create policy sg_poweruser_access_policy on repo for all to poweruser using (true);
CREATE POLICY
Time: 8.525 ms

# set role poweruser;
SET
Time: 0.338 ms

# select count(1) from repo;
┌───────┐
│ count │
╞═══════╡
│   552 │
└───────┘
(1 row)

Time: 5.782 ms
```
