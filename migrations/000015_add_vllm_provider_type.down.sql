-- Remove vllm from provider_type check constraint
ALTER TABLE providers DROP CONSTRAINT IF EXISTS providers_provider_type_check;
ALTER TABLE providers ADD CONSTRAINT providers_provider_type_check 
    CHECK (provider_type IN ('ollama', 'exolabs', 'llama_cpp')); 