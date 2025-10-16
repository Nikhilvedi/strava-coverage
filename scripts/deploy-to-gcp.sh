#!/bin/bash

# Strava Coverage - One-Click GCP Deployment Script
# =================================================

set -e  # Exit on any error

echo "🚀 Strava Coverage - GCP Deployment Setup"
echo "=========================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if gcloud is installed
if ! command -v gcloud &> /dev/null; then
    echo -e "${RED}❌ Google Cloud CLI not found. Please install it first:${NC}"
    echo "   https://cloud.google.com/sdk/docs/install"
    exit 1
fi

# Get project info
echo -e "${BLUE}📋 Project Configuration${NC}"
echo "========================="
read -p "Enter your GCP Project ID: " PROJECT_ID
read -p "Enter your Strava Client ID: " STRAVA_CLIENT_ID
read -s -p "Enter your Strava Client Secret: " STRAVA_CLIENT_SECRET
echo ""
read -p "Enter a secure database password: " DB_PASSWORD
echo ""
echo -e "${BLUE}💡 Quick Setup Options:${NC}"
read -p "Skip Cloud SQL instance check? (y/n) [n]: " SKIP_SQL_CHECK
SKIP_SQL_CHECK=${SKIP_SQL_CHECK:-n}
echo ""

# Pre-flight checks
echo -e "${BLUE}🔍 Pre-flight Checks${NC}"
echo "==================="

# Validate project ID format
if [[ ! $PROJECT_ID =~ ^[a-z][a-z0-9-]{4,28}[a-z0-9]$ ]]; then
    echo -e "${YELLOW}⚠️ Project ID format looks unusual. Should be lowercase letters, numbers, and hyphens.${NC}"
    echo -e "${YELLOW}Example: strava-coverage-123456${NC}"
    read -p "Continue anyway? (y/n): " continue_anyway
    if [[ ! $continue_anyway =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Set project first
echo -e "${BLUE}🔧 Setting up GCP project: $PROJECT_ID${NC}"
gcloud config set project $PROJECT_ID

# Verify we can access the project
echo -e "${BLUE}🔍 Verifying project access...${NC}"
if ! gcloud projects describe $PROJECT_ID &> /dev/null; then
    echo -e "${RED}❌ Cannot access project '$PROJECT_ID'${NC}"
    echo -e "${YELLOW}Please check:${NC}"
    echo "1. Project ID is correct (not project name or number)"
    echo "2. You have permission to access this project"
    echo "3. You are logged in: gcloud auth login"
    exit 1
fi
echo -e "${GREEN}✅ Project access verified${NC}"

# Check if instance already exists (with timeout)
if [[ $SKIP_SQL_CHECK =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}⚠️ Skipping Cloud SQL instance check (as requested)${NC}"
    echo -e "${BLUE}💡 Assuming instance needs to be created${NC}"
    SKIP_INSTANCE_CREATION=false
else
    echo -e "${BLUE}🔍 Checking Cloud SQL instance...${NC}"

# Create a temporary script for the gcloud command to handle timeout better
check_instance() {
    gcloud sql instances describe strava-coverage-db --project=$PROJECT_ID --format="value(state)" 2>/dev/null
}

# Check with timeout using a background process
TEMP_OUTPUT=$(mktemp)
check_instance > "$TEMP_OUTPUT" &
CHECK_PID=$!

# Wait for the command with timeout
TIMEOUT_SECONDS=15
ELAPSED=0
while [ $ELAPSED -lt $TIMEOUT_SECONDS ]; do
    if ! kill -0 $CHECK_PID 2>/dev/null; then
        # Process has finished
        wait $CHECK_PID
        INSTANCE_CHECK_RESULT=$?
        break
    fi
    sleep 1
    ELAPSED=$((ELAPSED + 1))
done

# If still running, kill it
if kill -0 $CHECK_PID 2>/dev/null; then
    echo -e "${YELLOW}⚠️ Cloud SQL check timed out after ${TIMEOUT_SECONDS}s${NC}"
    kill $CHECK_PID 2>/dev/null
    INSTANCE_CHECK_RESULT=124
fi

# Check results
if [ $INSTANCE_CHECK_RESULT -eq 0 ] && [ -s "$TEMP_OUTPUT" ]; then
    INSTANCE_STATE=$(cat "$TEMP_OUTPUT")
    echo -e "${GREEN}✅ Cloud SQL instance already exists (State: $INSTANCE_STATE)${NC}"
    SKIP_INSTANCE_CREATION=true
else
    echo -e "${YELLOW}⚠️ Cloud SQL instance will need to be created (5-15 minutes)${NC}"
    echo -e "${BLUE}💡 Tip: You can create it manually in Google Cloud Console if preferred${NC}"
    SKIP_INSTANCE_CREATION=false
fi

# Clean up temp file
rm -f "$TEMP_OUTPUT"
fi
echo ""

# Enable APIs
echo -e "${BLUE}🔧 Enabling required APIs...${NC}"
gcloud services enable \
  cloudbuild.googleapis.com \
  run.googleapis.com \
  sql-component.googleapis.com \
  secretmanager.googleapis.com

# Create Cloud SQL instance
echo -e "${BLUE}🗄️ Setting up Cloud SQL instance...${NC}"
if [ "$SKIP_INSTANCE_CREATION" = "false" ]; then
    echo -e "${YELLOW}⏳ This may take 5-15 minutes. Please be patient...${NC}"
    echo -e "${BLUE}💡 You can monitor progress in another terminal with:${NC}"
    echo -e "   ${YELLOW}gcloud sql operations list --filter='targetId:strava-coverage-db'${NC}"
    echo ""
    
    # Start the creation with async flag and monitor
    echo -e "${BLUE}🚀 Starting Cloud SQL instance creation (async)...${NC}"
    gcloud sql instances create strava-coverage-db \
      --database-version=POSTGRES_15 \
      --tier=db-f1-micro \
      --region=us-central1 \
      --storage-size=10GB \
      --storage-type=SSD \
      --project=$PROJECT_ID \
      --async
    
    echo -e "${BLUE}⏳ Waiting for instance to be ready...${NC}"
    
    # Wait for instance to be ready with progress indicators
    local attempts=0
    local max_attempts=60  # 30 minutes max (30 second intervals)
    
    while [ $attempts -lt $max_attempts ]; do
        if gcloud sql instances describe strava-coverage-db --project=$PROJECT_ID --format="value(state)" 2>/dev/null | grep -q "RUNNABLE"; then
            echo -e "\n${GREEN}✅ Cloud SQL instance created and ready!${NC}"
            break
        fi
        
        echo -n "."
        sleep 30
        attempts=$((attempts + 1))
        
        # Show progress every 2 minutes
        if [ $((attempts % 4)) -eq 0 ]; then
            echo -e "\n${BLUE}💭 Still creating... ($((attempts * 30 / 60)) minutes elapsed)${NC}"
        fi
    done
    
    if [ $attempts -eq $max_attempts ]; then
        echo -e "\n${RED}⚠️ Instance creation is taking longer than expected.${NC}"
        echo -e "${YELLOW}You can continue the setup later or check the Google Cloud Console.${NC}"
        echo -e "${BLUE}Continue anyway? (y/n):${NC}"
        read -r continue_anyway
        if [[ ! $continue_anyway =~ ^[Yy]$ ]]; then
            echo -e "${RED}Exiting. You can run this script again later.${NC}"
            exit 1
        fi
    fi
else
    echo -e "${GREEN}✅ Cloud SQL instance already exists - skipping creation${NC}"
fi

# Create database
echo -e "${BLUE}🏗️ Creating database...${NC}"
gcloud sql databases create strava_coverage --instance=strava-coverage-db --project=$PROJECT_ID || echo "Database might already exist"

# Create user
echo -e "${BLUE}👤 Creating database user...${NC}"
gcloud sql users create stravauser \
  --instance=strava-coverage-db \
  --password=$DB_PASSWORD \
  --project=$PROJECT_ID || echo "User might already exist"

# Create secrets
echo -e "${BLUE}🔐 Creating secrets...${NC}"
echo -n "$STRAVA_CLIENT_ID" | gcloud secrets create strava-client-id --project=$PROJECT_ID --data-file=- || \
  echo -n "$STRAVA_CLIENT_ID" | gcloud secrets versions add strava-client-id --project=$PROJECT_ID --data-file=-

echo -n "$STRAVA_CLIENT_SECRET" | gcloud secrets create strava-client-secret --project=$PROJECT_ID --data-file=- || \
  echo -n "$STRAVA_CLIENT_SECRET" | gcloud secrets versions add strava-client-secret --project=$PROJECT_ID --data-file=-

# We'll update the redirect URI after we know the Cloud Run URL
DB_CONNECTION="postgres://stravauser:$DB_PASSWORD@/strava_coverage?host=/cloudsql/$PROJECT_ID:us-central1:strava-coverage-db&sslmode=disable"
echo -n "$DB_CONNECTION" | gcloud secrets create database-connection --project=$PROJECT_ID --data-file=- || \
  echo -n "$DB_CONNECTION" | gcloud secrets versions add database-connection --project=$PROJECT_ID --data-file=-

# Create service account
echo -e "${BLUE}🔑 Creating service account...${NC}"
gcloud iam service-accounts create github-actions \
  --project=$PROJECT_ID \
  --display-name="GitHub Actions Deployment" || echo "Service account might already exist"

# Grant permissions
echo -e "${BLUE}🔐 Setting up permissions...${NC}"
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.admin"

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/storage.admin"

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/cloudsql.client"

# Create service account key
echo -e "${BLUE}🗝️ Creating service account key...${NC}"
gcloud iam service-accounts keys create github-sa-key.json \
  --project=$PROJECT_ID \
  --iam-account=github-actions@$PROJECT_ID.iam.gserviceaccount.com

echo ""
echo -e "${GREEN}🎉 GCP Setup Complete!${NC}"
echo "======================"
echo ""
echo -e "${BLUE}📋 Next Steps:${NC}"
echo "1. Add these secrets to your GitHub repository:"
echo "   - Go to: https://github.com/your-username/strava-coverage/settings/secrets/actions"
echo ""
echo -e "${YELLOW}   GCP_PROJECT_ID${NC} = $PROJECT_ID"
echo -e "${YELLOW}   CLOUDSQL_INSTANCE${NC} = $PROJECT_ID:us-central1:strava-coverage-db"
echo -e "${YELLOW}   GCP_SA_KEY${NC} = $(cat github-sa-key.json)"
echo ""
echo "2. Push your code to trigger deployment:"
echo "   git add ."
echo "   git commit -m 'Deploy to GCP'"
echo "   git push origin main"
echo ""
echo "3. After deployment, get your Cloud Run URL:"
echo "   gcloud run services list"
echo ""
echo "4. Update Strava OAuth settings with your new callback URL:"
echo "   https://your-cloud-run-url/oauth/callback"
echo ""
echo "5. Run database setup:"
echo "   ./scripts/setup-gcp-database.sh"
echo ""
echo -e "${GREEN}🚀 Your app will be live after pushing to GitHub!${NC}"

# Clean up
rm -f github-sa-key.json