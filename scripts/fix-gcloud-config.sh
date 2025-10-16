#!/bin/bash

# GCP Configuration Troubleshooter
# ================================

echo "🔧 GCP Configuration Troubleshooter"
echo "===================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}📋 Current gcloud configuration:${NC}"
echo "================================"

# Check if gcloud is installed
if ! command -v gcloud &> /dev/null; then
    echo -e "${RED}❌ gcloud CLI not installed${NC}"
    echo "Install from: https://cloud.google.com/sdk/docs/install"
    exit 1
fi

# Show current account
echo -e "${BLUE}👤 Current account:${NC}"
CURRENT_ACCOUNT=$(gcloud config get-value account 2>/dev/null)
if [ -z "$CURRENT_ACCOUNT" ]; then
    echo -e "${RED}❌ No account configured${NC}"
    echo -e "${YELLOW}Run: gcloud auth login${NC}"
else
    echo -e "${GREEN}✅ $CURRENT_ACCOUNT${NC}"
fi

# Show current project
echo -e "\n${BLUE}📂 Current project:${NC}"
CURRENT_PROJECT=$(gcloud config get-value project 2>/dev/null)
if [ -z "$CURRENT_PROJECT" ]; then
    echo -e "${RED}❌ No project configured${NC}"
else
    # Check if it's a project number vs project ID
    if [[ $CURRENT_PROJECT =~ ^[0-9]+$ ]]; then
        echo -e "${RED}❌ Project is set to PROJECT NUMBER: $CURRENT_PROJECT${NC}"
        echo -e "${YELLOW}This causes the error you're seeing!${NC}"
        
        # Try to get the project ID
        echo -e "\n${BLUE}🔍 Finding your project ID...${NC}"
        PROJECT_ID=$(gcloud projects list --filter="projectNumber:$CURRENT_PROJECT" --format="value(projectId)" 2>/dev/null)
        if [ -n "$PROJECT_ID" ]; then
            echo -e "${GREEN}✅ Your project ID is: $PROJECT_ID${NC}"
            echo -e "\n${BLUE}🔧 To fix the issue:${NC}"
            echo "gcloud config set project $PROJECT_ID"
        else
            echo -e "${RED}❌ Could not determine project ID${NC}"
        fi
    else
        echo -e "${GREEN}✅ Project ID: $CURRENT_PROJECT${NC}"
        
        # Verify we can access the project
        if gcloud projects describe $CURRENT_PROJECT &> /dev/null; then
            echo -e "${GREEN}✅ Project is accessible${NC}"
        else
            echo -e "${RED}❌ Cannot access project${NC}"
            echo -e "${YELLOW}Check permissions or project ID${NC}"
        fi
    fi
fi

# Show available projects
echo -e "\n${BLUE}📋 Available projects:${NC}"
echo "======================"
gcloud projects list --format="table(projectId,name,projectNumber)" 2>/dev/null || echo -e "${RED}❌ Cannot list projects${NC}"

# Show current region/zone
echo -e "\n${BLUE}🌍 Default region/zone:${NC}"
REGION=$(gcloud config get-value compute/region 2>/dev/null)
ZONE=$(gcloud config get-value compute/zone 2>/dev/null)
echo "Region: ${REGION:-not set}"
echo "Zone: ${ZONE:-not set}"

echo -e "\n${BLUE}💡 Common fixes:${NC}"
echo "================="

if [[ $CURRENT_PROJECT =~ ^[0-9]+$ ]]; then
    echo -e "${YELLOW}1. Fix project configuration:${NC}"
    if [ -n "$PROJECT_ID" ]; then
        echo "   gcloud config set project $PROJECT_ID"
    else
        echo "   gcloud config set project YOUR_PROJECT_ID"
    fi
    echo ""
fi

echo -e "${YELLOW}2. If not logged in:${NC}"
echo "   gcloud auth login"
echo ""

echo -e "${YELLOW}3. List all projects to find yours:${NC}"
echo "   gcloud projects list"
echo ""

echo -e "${YELLOW}4. Set correct project:${NC}"
echo "   gcloud config set project YOUR_PROJECT_ID"
echo ""

echo -e "${YELLOW}5. Verify configuration:${NC}"
echo "   gcloud config list"
echo ""

echo -e "${GREEN}🎯 After fixing, try your script again!${NC}"