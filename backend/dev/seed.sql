-- Local demonstration records only. This file contains no credentials and owns no schema.
-- Drizzle migrations remain the sole source of truth for database structure.

INSERT INTO users (id, auth_provider, auth_subject, email, display_name)
VALUES (
  '11111111-1111-4111-8111-111111111111',
  'development',
  'local-owner',
  'owner@sprout.local',
  'Local owner'
)
ON CONFLICT DO NOTHING;

INSERT INTO workspaces (id, slug, name, created_by)
VALUES (
  'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
  'acme',
  'Acme studio',
  '11111111-1111-4111-8111-111111111111'
)
ON CONFLICT DO NOTHING;

INSERT INTO workspace_memberships (workspace_id, user_id, role)
VALUES (
  'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
  '11111111-1111-4111-8111-111111111111',
  'owner'
)
ON CONFLICT DO NOTHING;

INSERT INTO applications (
  id,
  workspace_id,
  name,
  slug,
  description,
  lifecycle,
  default_hostname,
  created_by
)
VALUES
  (
    '22222222-2222-4222-8222-222222222222',
    'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
    'Invoice approvals',
    'invoice-approvals',
    'Review, route, and approve vendor invoices with a small team.',
    'active',
    'invoice-acme.sprout.run',
    '11111111-1111-4111-8111-111111111111'
  ),
  (
    '33333333-3333-4333-8333-333333333333',
    'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
    'Hiring pipeline',
    'hiring-pipeline',
    'A focused recruiting board for the product team.',
    'active',
    'hiring-acme.sprout.run',
    '11111111-1111-4111-8111-111111111111'
  ),
  (
    '44444444-4444-4444-8444-444444444444',
    'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
    'Inventory tracker',
    'inventory-tracker',
    'Reconcile warehouse counts from weekly CSV uploads.',
    'active',
    'inventory-acme.sprout.run',
    '11111111-1111-4111-8111-111111111111'
  ),
  (
    '55555555-5555-4555-8555-555555555555',
    'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
    'Research library',
    'research-library',
    'A searchable home for customer calls and product notes.',
    'paused',
    'research-acme.sprout.run',
    '11111111-1111-4111-8111-111111111111'
  )
ON CONFLICT DO NOTHING;
