// OAuth Callback Fix Template
// Apply this pattern to your OAuth callback page

'use client';

import { useAuth } from '@/app/context/AuthContext';
import { useRouter, useSearchParams } from 'next/navigation';
import { useEffect, useCallback, useState } from 'react';

export default function OAuthCallback() {
  const { setUser } = useAuth();
  const router = useRouter();
  const searchParams = useSearchParams();
  const [processed, setProcessed] = useState(false);

  // Memoize the callback handler to prevent infinite loops
  const handleCallback = useCallback(() => {
    // Prevent multiple executions
    if (processed) return;
    
    const success = searchParams.get('success');
    const userId = searchParams.get('user_id');
    const userName = searchParams.get('user_name');
    const stravaId = searchParams.get('strava_id');

    if (success === 'true' && userId && stravaId) {
      const userData = {
        id: parseInt(userId),
        stravaId: parseInt(stravaId),
        name: userName || 'Strava User',
        email: '', // Not provided by Strava
      };

      setUser(userData);
      setProcessed(true);
      
      // Redirect to dashboard after a short delay
      setTimeout(() => {
        router.push('/dashboard');
      }, 1000);
    } else {
      // Handle error case
      console.error('OAuth callback failed');
      setTimeout(() => {
        router.push('/');
      }, 2000);
    }
  }, [searchParams, setUser, router, processed]);

  // Use effect with proper dependencies
  useEffect(() => {
    handleCallback();
  }, [handleCallback]);

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-md w-full bg-white rounded-lg shadow-md p-8 text-center">
        {processed ? (
          <>
            <div className="w-16 h-16 mx-auto mb-4 bg-green-100 rounded-full flex items-center justify-center">
              <svg className="w-8 h-8 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
              </svg>
            </div>
            <h1 className="text-2xl font-bold text-gray-900 mb-2">Success!</h1>
            <p className="text-gray-600 mb-4">
              Your Strava account has been connected successfully.
            </p>
            <p className="text-sm text-gray-500">
              Redirecting to dashboard...
            </p>
          </>
        ) : (
          <>
            <div className="animate-spin rounded-full h-16 w-16 border-b-2 border-orange-500 mx-auto mb-4"></div>
            <h1 className="text-2xl font-bold text-gray-900 mb-2">Processing...</h1>
            <p className="text-gray-600">
              Completing your Strava connection...
            </p>
          </>
        )}
      </div>
    </div>
  );
}