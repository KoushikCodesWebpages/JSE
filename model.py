from transformers import pipeline, AutoModelForSeq2SeqLM, AutoTokenizer
import torch

model_name = "t5-small"  # or distilbart-cnn-12-6

tokenizer = AutoTokenizer.from_pretrained(model_name)
model = AutoModelForSeq2SeqLM.from_pretrained(model_name)

device = torch.device("cpu")  # Force CPU
summarizer = pipeline("text2text-generation", model=model, tokenizer=tokenizer, device=-1)

def summarize_job_description(description):
    prompt = f"Extract structured job details in JSON with fields: job_type, skills, description. Description: {description}"
    result = summarizer(prompt, max_length=512, do_sample=False)
    return result[0]['generated_text']

# Example
desc = "We are looking for a remote full stack developer with experience in React and Node.js..."
print(summarize_job_description(desc))
