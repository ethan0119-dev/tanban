ALTER TABLE product_option_values
  DROP FOREIGN KEY fk_product_option_attribute_value,
  DROP KEY idx_product_option_attribute_value,
  DROP COLUMN attribute_value_id;

ALTER TABLE product_option_groups
  DROP FOREIGN KEY fk_product_option_attribute_group,
  DROP KEY idx_product_option_attribute_group,
  DROP COLUMN attribute_group_id;

DROP TABLE IF EXISTS attribute_values;
DROP TABLE IF EXISTS attribute_groups;
