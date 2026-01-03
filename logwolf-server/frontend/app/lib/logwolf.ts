import Logwolf from '@jpricardo/logwolf-client-js';

export const logwolf = new Logwolf({ url: process.env.API_URL!, sampleRate: 0.5, errorSampleRate: 1 });
