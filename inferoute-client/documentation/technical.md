
## Overview

The Provider Client is a lightweight Go service that runs on Ollama provider machines. It will contain the following and should not be made into a microservice - I would like all the functions to be in a single go application. 


	•	Health Monitoring & Reporting:


1. It collects local metrics (e.g., GPU type, number of GPUs, utilization stats, models available) and reports them to the central system. It should send those requests to http://localhost:80/api/provider/health.


  To get the GPU details you can run nvidia-smi -x-g and extract the GPU type, GPU driver version etc.... the nvidia-smi.xml file contains an example of what the XML ouput of that command looks like.

  To get the list of models available (for now we only support ollama, but we wll in future support ther providers)  you can connect to the the local ollama istance via the REST API and call /v1/models.

This should be send every 5 minutes, but there should also be an API called /health that a provider can manually call to get all this info.

2. It should also expose an API called /busy that also runs nvidia-smi -x-g and quickly checks whether the GPU is busy. This should only return TRUE or FALSE.



	•	Inference Request Handling:
When an inference request is received from the central orchestrator, the provider client must first determine whether it's GPU's is currently busy by calling the /busy API route.

	•	If the GPU is busy:
The service will immediately respond back to the orchestrator (via HTTP) indicating that it is currently busy, so that the orchestrator can try another provider.
	•	If the GPU is available:
It forwards the request to the local inference engine (e.g., Ollama) and processes the inference.
	•	HMAC Validation:
	Each request to a provider will contain a HMAC and the provider should connect to the HMAC vlidaton API and confirm that the request is valid before moving forward. This should happen after it's determied that teh GPU is free.
Validates HMACs on incoming requests when necessary. See the provider-management microservice on how to validate an HMAC the API route will be /api/provider/validate_hmac ( the IP address to use will be the one that made the request to the provider)
	•	NGROK Tunnel Integration:
The provider client should include ngrok to create a secure tunnel back to the central system. This allows the provider’s local inference engine (or Ollama server) to be accessible via a public URL without exposing the machine directly. This public URL should also be send in the health update.


	•	The go application should read from a configuration file, where we can keep some settings. Like the local ollama address and port. defaults to http://localhost:11434. 

Key Points for the Provider Client
	1.	Local Health Check (/health):
	•	This endpoint returns JSON data that includes details such as current GPU utilization, busy status, and other metrics.
	•	The orchestrator will call this endpoint before dispatching an inference request. If the GPU is busy, the provider client must promptly inform the orchestrator.
	2.	NGROK Tunnel Setup:
	•	The Docker container for the provider client will install the ngrok package.
	•	This package creates a tunnel so that the provider’s local endpoints (e.g., for inference requests) are exposed via a secure public URL.
	•	The public URL should be included in the health report data for use by the orchestrator.
	3.	Ease of Deployment:
	•	The provider client is built in Go, packaged in a Docker container, and designed to be as small and efficient as possible.






