-- Reverses 0002_products. Leaves flowos (owned by 0001_init).
DELETE FROM products
WHERE code IN ('optionalyzer', 'dhansanketai', 'pashutrack', 'teleflow');
