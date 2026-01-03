import Logwolf from '@jpricardo/logwolf-client-js';
import z from 'zod';

export const logwolf = new Logwolf(z.string().parse(process.env.API_URL));
