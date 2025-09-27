# Model Selection Documentation

You are documenting model selection strategies for grove-docgen.

## Task
Create a guide on choosing and configuring the right AI models for documentation generation.

## Topics to Cover

1. **Available Models**
   - Gemini models (gemini-2.5-pro, gemini-2.5-flash)
   - OpenAI models (gpt-4o, gpt-4o-mini)
   - Anthropic models (claude-3-5-sonnet, claude-3-5-haiku)
   - Model capabilities and limitations

2. **Selection Criteria**
   - Cost considerations per token
   - Speed vs quality trade-offs
   - Context window sizes
   - Output consistency requirements

3. **Model-Specific Optimization**
   - Best temperature ranges per model
   - Token limit considerations
   - Sampling parameter tuning
   - Response format handling

4. **Use Case Recommendations**
   - API reference: Low temperature, high consistency models
   - Tutorials: Balanced creativity and accuracy
   - Conceptual docs: Higher reasoning capability
   - Quick iterations: Fast, cost-effective models

5. **Per-Section Configuration**
   - Overriding global model settings
   - Mixing models in a single project
   - A/B testing different models
   - Migration strategies between models

## Practical Examples
Include specific recommendations for:
- Large codebases with extensive APIs
- Small libraries needing quick docs
- Tutorial-heavy documentation
- Technical reference materials

## Performance Metrics
Document typical generation times, costs, and quality indicators for each model option.