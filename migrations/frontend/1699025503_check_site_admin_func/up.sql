CREATE OR REPLACE FUNCTION is_user_site_admin(user_id INT) RETURNS BOOLEAN STABLE AS $$
    SELECT EXISTS (
        SELECT 1
        FROM user_roles ur
        JOIN roles r ON r.id = ur.role_id
        WHERE ur.user_id = is_user_site_admin.user_id
          AND r.name = 'SITE_ADMINISTRATOR'
    );
$$ LANGUAGE sql;
