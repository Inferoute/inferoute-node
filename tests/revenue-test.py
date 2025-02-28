def calculate_monthly_revenue():
    # Daily transaction parameters
    transactions_per_day = 100000000
    input_tokens_per_tx = 1000
    output_tokens_per_tx = 200
    days = 30
    fee_percentage = 0.10  # 10%

    # Model pricing from seed.sql (price per 1M tokens)
    models = {
        "Tier 1": {
            "gpt-4-turbo": {"input": 0.15, "output": 0.3},
            "claude-3-opus": {"input": 0.15, "output": 0.35},
            "gemini-pro": {"input": 0.8, "output": 0.25}
        },
        "Tier 2": {
            "gpt-3.5-turbo": {"input": 0.5, "output": 0.15},
            "claude-2": {"input": 0.6, "output": 0.18},
            "mistral-medium": {"input": 0.4, "output": 0.12}
        },
        "Tier 3": {
            "deepseek-r1:32b": {"input": 0.2, "output": 0.6},
            "llama-2": {"input": 0.15, "output": 0.45}
        }
    }

    total_transactions = transactions_per_day * days
    print(f"\nCalculating revenue for {days} days:")
    print(f"Daily transactions: {transactions_per_day}")
    print(f"Input tokens per tx: {input_tokens_per_tx}")
    print(f"Output tokens per tx: {output_tokens_per_tx}")
    print(f"Total transactions: {total_transactions}")
    print(f"Fee percentage: {fee_percentage*100}%\n")

    total_revenue = 0

    for tier, tier_models in models.items():
        print(f"\n{tier}:")
        for model, prices in tier_models.items():
            # Calculate cost per transaction
            input_cost = (input_tokens_per_tx * prices['input']) / 1_000_000
            output_cost = (output_tokens_per_tx * prices['output']) / 1_000_000
            
            # Calculate total costs and our revenue
            cost_per_tx = input_cost + output_cost
            total_cost = cost_per_tx * total_transactions
            our_revenue = total_cost * fee_percentage

            print(f"\n{model}:")
            print(f"  Cost per tx: ${cost_per_tx:.4f}")
            print(f"  Monthly transaction volume: ${total_cost:.2f}")
            print(f"  Our revenue (5%): ${our_revenue:.2f}")
            
            total_revenue += our_revenue

    print(f"\nTotal potential monthly revenue: ${total_revenue:.2f}")
    print(f"Average revenue per model: ${total_revenue/len([m for t in models.values() for m in t]):.2f}")

if __name__ == "__main__":
    calculate_monthly_revenue()