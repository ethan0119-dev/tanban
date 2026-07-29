ALTER TABLE printer_devices
  ADD COLUMN label_width_mm INT NULL AFTER paper_width,
  ADD COLUMN label_height_mm INT NULL AFTER label_width_mm;

UPDATE printer_devices
SET label_width_mm=40,label_height_mm=30
WHERE output_type='LABEL'
  AND UPPER(REPLACE(model,' ','')) IN ('XP-T271U','T271U')
  AND (label_width_mm IS NULL OR label_height_mm IS NULL);
