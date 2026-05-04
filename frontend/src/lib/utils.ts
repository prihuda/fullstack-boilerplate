import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

type ClassValue = string | number | boolean | undefined | null | ClassValue[];

export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}

