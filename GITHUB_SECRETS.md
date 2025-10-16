# GitHub Secrets Setup Guide

## Required Secrets for Deployment

### Backend Repository: `strava-coverage`

Go to: `https://github.com/YOUR_USERNAME/strava-coverage/settings/secrets/actions`

Add these secrets:

| Secret Name | Description | Example Value |
|-------------|-------------|---------------|
| `GCP_PROJECT_ID` | Your Google Cloud Project ID | `strava-coverage-123456` |
| `GCP_SA_KEY` | Service Account JSON Key | `{"type": "service_account", "project_id": "..."}` |
| `CLOUDSQL_INSTANCE` | Cloud SQL Instance Connection | `your-project:us-central1:strava-coverage-db` |

### Frontend Repository: `strava-coverage-frontend`

Go to: `https://github.com/YOUR_USERNAME/strava-coverage-frontend/settings/secrets/actions`

Add these secrets:

| Secret Name | Description | Example Value |
|-------------|-------------|---------------|
| `GCP_PROJECT_ID` | Your Google Cloud Project ID | `strava-coverage-123456` |
| `GCP_SA_KEY` | Service Account JSON Key | `{"type": "service_account", "project_id": "..."}` |
| `NEXT_PUBLIC_API_URL` | Backend API URL | `https://strava-coverage-backend-xxx.run.app` |

## How to Get the Service Account Key

### Option 1: Using the Setup Script (Recommended)

```bash
# Run the deployment setup script
./scripts/deploy-to-gcp.sh
```

This script will:
1. Create the service account
2. Generate the JSON key
3. Show you the exact values to copy

### Option 2: Manual Setup

```bash
# 1. Create service account
gcloud iam service-accounts create github-actions \
  --display-name="GitHub Actions Deployment"

# 2. Grant permissions
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:github-actions@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.admin"

gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:github-actions@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/storage.admin"

gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:github-actions@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

# 3. Create and download the key
gcloud iam service-accounts keys create github-sa-key.json \
  --iam-account=github-actions@YOUR_PROJECT_ID.iam.gserviceaccount.com

# 4. Copy the contents of github-sa-key.json
cat github-sa-key.json
```

## Common Issues and Solutions

### ❌ "must specify exactly one of workload_identity_provider or credentials_json"

**This error means the GCP_SA_KEY secret is missing or incorrectly formatted.**

**Quick Fix**: Use the secrets generator script:
```bash
./scripts/generate-github-secrets.sh
```

**If you get "project number" error**:
```bash
./scripts/fix-gcloud-config.sh
```

**Manual Check**: The `GCP_SA_KEY` secret must contain valid JSON like:
```json
{
  "type": "service_account",
  "project_id": "your-project-id",
  "private_key_id": "...",
  "private_key": "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n",
  "client_email": "github-actions@your-project-id.iam.gserviceaccount.com",
  "client_id": "...",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/github-actions%40your-project-id.iam.gserviceaccount.com"
}
```

**Common Issues**:
- ❌ Secret is empty or not set
- ❌ Secret contains only part of the JSON
- ❌ Secret has extra spaces or newlines
- ❌ Secret is missing quotes or brackets

### ❌ "The project property must be set to a valid project ID"

**Solution**: Use the project ID (like `strava-coverage-123456`), not the project name (like `Strava Coverage`).

### ❌ "Permission denied" errors

**Solution**: Make sure the service account has all required roles:
- `roles/run.admin` - For Cloud Run deployment
- `roles/storage.admin` - For Container Registry
- `roles/secretmanager.secretAccessor` - For accessing secrets
- `roles/cloudsql.client` - For database connections

## Testing the Setup

After adding the secrets, you can test by:

1. **Triggering the workflow manually**:
   - Go to the Actions tab in GitHub
   - Select "Deploy [Backend/Frontend] to GCP Cloud Run"
   - Click "Run workflow"

2. **Making a commit**:
   ```bash
   git add .
   git commit -m "Test deployment"
   git push origin main
   ```

3. **Check the logs**:
   - Go to the Actions tab
   - Click on the running workflow
   - Check each step for errors

## Success Indicators

✅ **Authentication step shows**: "Authenticated as github-actions@your-project.iam.gserviceaccount.com"

✅ **Verification step shows**: Your project ID

✅ **Build step completes**: Docker image pushed successfully

✅ **Deploy step shows**: Service deployed URL

✅ **Smoke test passes**: Health check responds correctly