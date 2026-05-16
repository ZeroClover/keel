# web-dashboard Specification

## Purpose
TBD - created by archiving change remove-approvals-system. Update Purpose after archive.
## Requirements
### Requirement: Dashboard Route Set

The Vue web dashboard SHALL remove the `/approvals` route and MUST NOT load `ui/src/views/approvals/Approvals.vue`. Existing non-approval routes such as `/dashboard`, `/tracked-images`, `/audit-logs`, `/user/login`, and `/404` MUST keep their current paths unless another Change explicitly renames them.

#### Scenario: User navigates to /approvals

- **WHEN** a user opens `https://<keel-host>/approvals` in a browser
- **THEN** Vue Router MUST resolve to the application's catch-all "not found" handler (no `Approvals` component is loaded)

### Requirement: Dashboard Cards and Columns

The Dashboard analysis view MUST NOT display:

1. A "Pending Approvals" chart card.
2. A "Required Approvals" column in the tracked-resources table.
3. Approve/Reject action buttons next to each tracked resource.
4. A "Policy & Approvals Control" column title (replaced by "Policy" alone).

#### Scenario: Dashboard loads without approval widgets

- **WHEN** the dashboard `/dashboard/analysis` view renders
- **THEN** no DOM element with text "Pending Approvals", "Required Approvals", or "Approve" / "Reject" button MUST be present
- **AND** the policy column title MUST read exactly "Policy"

### Requirement: Store Module Surface

The Vuex store SHALL NOT register an `approvals` module. The existing approval getters `approvalsPending`, `approvalsApprovedCount`, and `approvalsRejectedCount` MUST NOT exist. The action dispatchers `GetApprovals`, `UpdateApproval`, `SetApproval` MUST NOT be defined.

#### Scenario: Build emits no approval store references

- **WHEN** the UI is built with `yarn run build`
- **THEN** the produced bundles MUST NOT contain string literals `GetApprovals`, `UpdateApproval`, `SetApproval`
- **AND** the bundle MUST NOT import any module from `@/store/modules/approvals`
