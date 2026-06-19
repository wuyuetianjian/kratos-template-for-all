# Auth Permission Design

## Goals

The permission system is database-driven. Roles, permissions, modules, role
inheritance, and user-role bindings are stored in the database and managed by
APIs. Code must not hard-code business permission names.

System integrity rules are enforced in code:

- The default `admin` user cannot be deleted.
- The current authenticated user cannot delete itself.
- The default `Admin` role cannot be deleted.
- The `admin` user must keep the `Admin` role.
- Role inheritance cannot contain cycles.

## Default Bootstrap

On startup, the data layer ensures these records exist:

- Module: `system`
- Permission: `system:*`
- Role: `Admin`
- User: `admin`

`Admin` is the system administrator role and grants full read/write access. The
initial `admin` password is generated at startup when the default admin user is
created or when the previous initialization password has never been used and no
plaintext password is available in process memory. The password hash is stored
in the database; plaintext is only printed to logs and retained in process
memory until first successful login.

The frontend initialization page can call the unauthenticated
`GET /v1/auth/initial-password` endpoint. While the default admin initialization
password is unused and still available in the current process, the endpoint
returns `available=true`, `username=admin`, and the plaintext initialization
password. After the first successful admin login marks the initialization
password as used, the endpoint returns `available=false` and no password.

When `admin` logs in with the initialization password before it has been used,
the login response returns the plaintext initialization password and
`must_change_password=true`. After that login, the password is marked used and
is never returned again.

## Permission Levels

Permissions are modeled as `module + action`.

- Level 1: `Admin`, full access through `system:*`.
- Level 2: module administrator roles, such as `module:manage`.
- Level 3: module read-only and special function permissions, such as
  `module:read` or `module:special-action`.

The concrete module and action values are database data. API handlers and
middleware only evaluate stored permissions.

## Role Inheritance

Custom roles can inherit permissions from other roles. Effective permissions
are the union of:

- permissions directly assigned to the role;
- permissions assigned to inherited roles, recursively.

The role inheritance table stores parent-child relationships. A child role
inherits from a parent role. Before saving inheritance, the system validates
that the graph remains acyclic.

## User And Role Management

Administrators can manage users, roles, permissions, and role inheritance:

- Create, list, get, update, and delete users.
- Assign roles to users.
- Create, list, get, update, and delete roles.
- Assign permissions to roles.
- Set role inheritance.
- Create, list, get, update, and delete permissions.

Delete operations enforce system integrity rules even if the caller has broad
permissions.

## API Surface

Public endpoints:

- `POST /v1/auth/login`
- `GET /v1/auth/initial-password`
- `GET /health`

Authenticated endpoints:

- `POST /v1/auth/change-password`
- `GET /v1/auth/me`
- `POST /v1/users`
- `GET /v1/users`
- `GET /v1/users/{id}`
- `PATCH /v1/users/{id}`
- `DELETE /v1/users/{id}`
- `PUT /v1/users/{user_id}/roles`
- `POST /v1/roles`
- `GET /v1/roles`
- `GET /v1/roles/{id}`
- `PATCH /v1/roles/{id}`
- `DELETE /v1/roles/{id}`
- `PUT /v1/roles/{role_id}/permissions`
- `PUT /v1/roles/{role_id}/inheritances`
- `POST /v1/permissions`
- `GET /v1/permissions`
- `PATCH /v1/permissions/{id}`
- `DELETE /v1/permissions/{id}`

## Database Tables

Ent creates the permission tables under `internal/data/ent`:

- `auth_users`
- `auth_roles`
- `auth_modules`
- `auth_permissions`
- `auth_user_roles`
- `auth_role_permissions`
- `auth_role_parents`

`auth_role_parents` stores role inheritance. A role inherits permissions from every
parent role recursively.

## Configuring Permissions

Permission records are created through APIs and stored in the database. A
permission can target a specific Kratos operation with `operation`, for example:

```json
{
  "module": "users",
  "action": "manage",
  "operation": "/temperate.v1.TemperateService/CreateUser",
  "description": "Create users"
}
```

Wildcard operation prefixes are supported by storing an operation ending in
`*`, for example `/temperate.v1.TemperateService/*`. The seeded `Admin` role has
`system:*`, which allows all operations.

## Request Authorization

Authentication uses JWT. Login issues a JWT containing `user_id` and
`username`. Server middleware parses the token, loads the user and effective
permissions from the database, and authorizes the current operation.

Public operations are code-level allowlisted:

- `Health`
- `Login`

All other operations require a valid token. Users with the `Admin` role or
`system:*` permission are allowed to call every operation. Other users must have
a permission whose `operation` matches the current Kratos operation or a
wildcard permission such as `<module>:*`.
