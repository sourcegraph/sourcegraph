import React from 'react'
import type { ReactElement } from 'react'

import { Icon } from '@sourcegraph/wildcard'

export const codyIconPath =
    'm9 15a1 1 0 01-1-1v-2a1 1 0 012 0v2a1 1 0 01-1 1zm6 0a1 1 0 01-1-1v-2a1 1 0 012 0v2a1 1 0 01-1 1zm-9-7a1 1 0 01-.71-.29l-3-3a1 1 0 011.42-1.42l3 3a1 1 0 010 1.42 1 1 0 01-.71.29zm12 0a1 1 0 01-.71-.29 1 1 0 010-1.42l3-3a1 1 0 111.42 1.42l-3 3a1 1 0 01-.71.29zm3 12h-18a1 1 0 01-1-1v-4.5a10 10 0 0120 0v4.5a1 1 0 01-1 1zm-17-2h16v-3.5a8 8 0 00-16 0z'

export const CodyIcon: React.FunctionComponent<{ className?: string }> = ({ className }) => (
    <Icon svgPath={codyIconPath} className={className} aria-hidden={true} />
)

export const CodyProIcon = ({ className }: { className: string }): ReactElement => (
    <svg
        xmlns="http://www.w3.org/2000/svg"
        width="111"
        height="29"
        viewBox="0 0 111 29"
        fill="none"
        className={className}
    >
        <g filter="url(#filter0_d_3576_1250)">
            <path
                d="M19.7372 7.76562H15.3821C15.3026 7.20218 15.1402 6.7017 14.8949 6.2642C14.6496 5.82007 14.3348 5.44223 13.9503 5.13068C13.5658 4.81913 13.1217 4.58049 12.6179 4.41477C12.1207 4.24905 11.5805 4.16619 10.9972 4.16619C9.94318 4.16619 9.0251 4.42803 8.2429 4.9517C7.4607 5.46875 6.85417 6.22443 6.4233 7.21875C5.99242 8.20644 5.77699 9.40625 5.77699 10.8182C5.77699 12.2699 5.99242 13.4896 6.4233 14.4773C6.8608 15.465 7.47064 16.2107 8.25284 16.7145C9.03504 17.2183 9.93987 17.4702 10.9673 17.4702C11.544 17.4702 12.0777 17.3939 12.5682 17.2415C13.0653 17.089 13.5062 16.867 13.8906 16.5753C14.2751 16.277 14.5933 15.9157 14.8452 15.4915C15.1037 15.0672 15.2827 14.5833 15.3821 14.0398L19.7372 14.0597C19.6245 14.9943 19.3428 15.8958 18.892 16.7642C18.4479 17.6259 17.848 18.3982 17.0923 19.081C16.3433 19.7571 15.4484 20.294 14.4077 20.6918C13.3736 21.0829 12.2036 21.2784 10.8977 21.2784C9.08144 21.2784 7.45739 20.8674 6.02557 20.0455C4.60038 19.2235 3.47348 18.0336 2.64489 16.4759C1.82292 14.9181 1.41193 13.0322 1.41193 10.8182C1.41193 8.59754 1.82955 6.70833 2.66477 5.15057C3.5 3.5928 4.63352 2.40625 6.06534 1.59091C7.49716 0.768939 9.10795 0.357954 10.8977 0.357954C12.0777 0.357954 13.1714 0.523674 14.179 0.855113C15.1932 1.18655 16.0914 1.67045 16.8736 2.30682C17.6558 2.93655 18.2921 3.70881 18.7827 4.62358C19.2798 5.53835 19.598 6.5857 19.7372 7.76562ZM27.6456 21.2983C26.1011 21.2983 24.7654 20.9702 23.6385 20.3139C22.5182 19.651 21.6532 18.7296 21.0433 17.5497C20.4335 16.3632 20.1286 14.9877 20.1286 13.4233C20.1286 11.8456 20.4335 10.4669 21.0433 9.28693C21.6532 8.10038 22.5182 7.17898 23.6385 6.52273C24.7654 5.85985 26.1011 5.52841 27.6456 5.52841C29.1901 5.52841 30.5225 5.85985 31.6428 6.52273C32.7696 7.17898 33.638 8.10038 34.2479 9.28693C34.8577 10.4669 35.1626 11.8456 35.1626 13.4233C35.1626 14.9877 34.8577 16.3632 34.2479 17.5497C33.638 18.7296 32.7696 19.651 31.6428 20.3139C30.5225 20.9702 29.1901 21.2983 27.6456 21.2983ZM27.6655 18.017C28.3681 18.017 28.9548 17.8182 29.4254 17.4205C29.8961 17.0161 30.2507 16.4659 30.4893 15.7699C30.7346 15.0739 30.8572 14.2817 30.8572 13.3935C30.8572 12.5052 30.7346 11.7131 30.4893 11.017C30.2507 10.321 29.8961 9.77083 29.4254 9.36648C28.9548 8.96212 28.3681 8.75994 27.6655 8.75994C26.9562 8.75994 26.3596 8.96212 25.8757 9.36648C25.3984 9.77083 25.0372 10.321 24.7919 11.017C24.5533 11.7131 24.4339 12.5052 24.4339 13.3935C24.4339 14.2817 24.5533 15.0739 24.7919 15.7699C25.0372 16.4659 25.3984 17.0161 25.8757 17.4205C26.3596 17.8182 26.9562 18.017 27.6655 18.017ZM41.5447 21.2486C40.3847 21.2486 39.334 20.9503 38.3928 20.3537C37.4581 19.7505 36.7157 18.8655 36.1655 17.6989C35.6219 16.5256 35.3501 15.0871 35.3501 13.3835C35.3501 11.6335 35.6319 10.1785 36.1953 9.01847C36.7588 7.8518 37.5078 6.98011 38.4425 6.40341C39.3838 5.82008 40.4145 5.52841 41.5348 5.52841C42.3899 5.52841 43.1025 5.67424 43.6726 5.96591C44.2493 6.25095 44.7133 6.6089 45.0646 7.03977C45.4226 7.46401 45.6944 7.88163 45.88 8.29261H46.0092V0.636363H50.2351V21H46.0589V18.554H45.88C45.6811 18.9782 45.3994 19.3991 45.0348 19.8168C44.6768 20.2277 44.2095 20.5691 43.6328 20.8409C43.0627 21.1127 42.3667 21.2486 41.5447 21.2486ZM42.8871 17.8778C43.5698 17.8778 44.1465 17.6922 44.6172 17.321C45.0945 16.9432 45.459 16.4162 45.7109 15.7401C45.9695 15.0639 46.0987 14.2718 46.0987 13.3636C46.0987 12.4555 45.9728 11.6667 45.7209 10.9972C45.469 10.3277 45.1044 9.81061 44.6271 9.44602C44.1499 9.08144 43.5698 8.89915 42.8871 8.89915C42.1911 8.89915 41.6044 9.08807 41.1271 9.46591C40.6499 9.84375 40.2886 10.3674 40.0433 11.0369C39.7981 11.7064 39.6754 12.482 39.6754 13.3636C39.6754 14.2519 39.7981 15.0374 40.0433 15.7202C40.2952 16.3963 40.6565 16.9266 41.1271 17.3111C41.6044 17.6889 42.1911 17.8778 42.8871 17.8778ZM54.3327 26.7273C53.7958 26.7273 53.292 26.6842 52.8214 26.598C52.3574 26.5185 51.9729 26.4157 51.668 26.2898L52.6225 23.1278C53.1197 23.2803 53.5671 23.3632 53.9648 23.3764C54.3692 23.3897 54.7172 23.2969 55.0089 23.098C55.3072 22.8991 55.5491 22.5611 55.7347 22.0838L55.9833 21.4375L50.5046 5.72727H54.9592L58.1211 16.9432H58.2802L61.4719 5.72727H65.9563L60.0202 22.6506C59.7352 23.4725 59.3474 24.1884 58.8569 24.7983C58.373 25.4148 57.7598 25.8887 57.0174 26.2202C56.275 26.5582 55.3801 26.7273 54.3327 26.7273ZM70.6839 21V0.636363H78.718C80.2625 0.636363 81.5784 0.931344 82.6655 1.52131C83.7526 2.10464 84.5812 2.91667 85.1513 3.95739C85.728 4.99148 86.0163 6.18466 86.0163 7.53693C86.0163 8.8892 85.7247 10.0824 85.1413 11.1165C84.558 12.1506 83.7128 12.956 82.6058 13.5327C81.5054 14.1094 80.1731 14.3977 78.6087 14.3977H73.4879V10.9474H77.9126C78.7412 10.9474 79.424 10.8049 79.9609 10.5199C80.5045 10.2282 80.9089 9.82718 81.174 9.31676C81.4458 8.79972 81.5817 8.20644 81.5817 7.53693C81.5817 6.86079 81.4458 6.27083 81.174 5.76704C80.9089 5.25663 80.5045 4.86222 79.9609 4.58381C79.4174 4.29877 78.728 4.15625 77.8928 4.15625H74.9893V21H70.6839ZM86.7333 21V5.72727H90.8398V8.39205H90.9989C91.2773 7.44413 91.7447 6.72822 92.4009 6.24432C93.0572 5.75379 93.8129 5.50852 94.668 5.50852C94.8801 5.50852 95.1088 5.52178 95.354 5.54829C95.5993 5.57481 95.8147 5.61127 96.0004 5.65767V9.41619C95.8015 9.35653 95.5264 9.3035 95.1751 9.2571C94.8237 9.2107 94.5022 9.1875 94.2106 9.1875C93.5875 9.1875 93.0307 9.32339 92.5401 9.59517C92.0562 9.86032 91.6718 10.2315 91.3867 10.7088C91.1083 11.1861 90.9691 11.7363 90.9691 12.3594V21H86.7333ZM102.38 21.2983C100.835 21.2983 99.4998 20.9702 98.3729 20.3139C97.2526 19.651 96.3875 18.7296 95.7777 17.5497C95.1679 16.3632 94.8629 14.9877 94.8629 13.4233C94.8629 11.8456 95.1679 10.4669 95.7777 9.28693C96.3875 8.10038 97.2526 7.17898 98.3729 6.52273C99.4998 5.85985 100.835 5.52841 102.38 5.52841C103.924 5.52841 105.257 5.85985 106.377 6.52273C107.504 7.17898 108.372 8.10038 108.982 9.28693C109.592 10.4669 109.897 11.8456 109.897 13.4233C109.897 14.9877 109.592 16.3632 108.982 17.5497C108.372 18.7296 107.504 19.651 106.377 20.3139C105.257 20.9702 103.924 21.2983 102.38 21.2983ZM102.4 18.017C103.103 18.017 103.689 17.8182 104.16 17.4205C104.63 17.0161 104.985 16.4659 105.224 15.7699C105.469 15.0739 105.592 14.2817 105.592 13.3935C105.592 12.5052 105.469 11.7131 105.224 11.017C104.985 10.321 104.63 9.77083 104.16 9.36648C103.689 8.96212 103.103 8.75994 102.4 8.75994C101.691 8.75994 101.094 8.96212 100.61 9.36648C100.133 9.77083 99.7715 10.321 99.5263 11.017C99.2876 11.7131 99.1683 12.5052 99.1683 13.3935C99.1683 14.2817 99.2876 15.0739 99.5263 15.7699C99.7715 16.4659 100.133 17.0161 100.61 17.4205C101.094 17.8182 101.691 18.017 102.4 18.017Z"
                fill="#EFF2F5"
            />
            <path
                d="M19.7372 7.76562H15.3821C15.3026 7.20218 15.1402 6.7017 14.8949 6.2642C14.6496 5.82007 14.3348 5.44223 13.9503 5.13068C13.5658 4.81913 13.1217 4.58049 12.6179 4.41477C12.1207 4.24905 11.5805 4.16619 10.9972 4.16619C9.94318 4.16619 9.0251 4.42803 8.2429 4.9517C7.4607 5.46875 6.85417 6.22443 6.4233 7.21875C5.99242 8.20644 5.77699 9.40625 5.77699 10.8182C5.77699 12.2699 5.99242 13.4896 6.4233 14.4773C6.8608 15.465 7.47064 16.2107 8.25284 16.7145C9.03504 17.2183 9.93987 17.4702 10.9673 17.4702C11.544 17.4702 12.0777 17.3939 12.5682 17.2415C13.0653 17.089 13.5062 16.867 13.8906 16.5753C14.2751 16.277 14.5933 15.9157 14.8452 15.4915C15.1037 15.0672 15.2827 14.5833 15.3821 14.0398L19.7372 14.0597C19.6245 14.9943 19.3428 15.8958 18.892 16.7642C18.4479 17.6259 17.848 18.3982 17.0923 19.081C16.3433 19.7571 15.4484 20.294 14.4077 20.6918C13.3736 21.0829 12.2036 21.2784 10.8977 21.2784C9.08144 21.2784 7.45739 20.8674 6.02557 20.0455C4.60038 19.2235 3.47348 18.0336 2.64489 16.4759C1.82292 14.9181 1.41193 13.0322 1.41193 10.8182C1.41193 8.59754 1.82955 6.70833 2.66477 5.15057C3.5 3.5928 4.63352 2.40625 6.06534 1.59091C7.49716 0.768939 9.10795 0.357954 10.8977 0.357954C12.0777 0.357954 13.1714 0.523674 14.179 0.855113C15.1932 1.18655 16.0914 1.67045 16.8736 2.30682C17.6558 2.93655 18.2921 3.70881 18.7827 4.62358C19.2798 5.53835 19.598 6.5857 19.7372 7.76562ZM27.6456 21.2983C26.1011 21.2983 24.7654 20.9702 23.6385 20.3139C22.5182 19.651 21.6532 18.7296 21.0433 17.5497C20.4335 16.3632 20.1286 14.9877 20.1286 13.4233C20.1286 11.8456 20.4335 10.4669 21.0433 9.28693C21.6532 8.10038 22.5182 7.17898 23.6385 6.52273C24.7654 5.85985 26.1011 5.52841 27.6456 5.52841C29.1901 5.52841 30.5225 5.85985 31.6428 6.52273C32.7696 7.17898 33.638 8.10038 34.2479 9.28693C34.8577 10.4669 35.1626 11.8456 35.1626 13.4233C35.1626 14.9877 34.8577 16.3632 34.2479 17.5497C33.638 18.7296 32.7696 19.651 31.6428 20.3139C30.5225 20.9702 29.1901 21.2983 27.6456 21.2983ZM27.6655 18.017C28.3681 18.017 28.9548 17.8182 29.4254 17.4205C29.8961 17.0161 30.2507 16.4659 30.4893 15.7699C30.7346 15.0739 30.8572 14.2817 30.8572 13.3935C30.8572 12.5052 30.7346 11.7131 30.4893 11.017C30.2507 10.321 29.8961 9.77083 29.4254 9.36648C28.9548 8.96212 28.3681 8.75994 27.6655 8.75994C26.9562 8.75994 26.3596 8.96212 25.8757 9.36648C25.3984 9.77083 25.0372 10.321 24.7919 11.017C24.5533 11.7131 24.4339 12.5052 24.4339 13.3935C24.4339 14.2817 24.5533 15.0739 24.7919 15.7699C25.0372 16.4659 25.3984 17.0161 25.8757 17.4205C26.3596 17.8182 26.9562 18.017 27.6655 18.017ZM41.5447 21.2486C40.3847 21.2486 39.334 20.9503 38.3928 20.3537C37.4581 19.7505 36.7157 18.8655 36.1655 17.6989C35.6219 16.5256 35.3501 15.0871 35.3501 13.3835C35.3501 11.6335 35.6319 10.1785 36.1953 9.01847C36.7588 7.8518 37.5078 6.98011 38.4425 6.40341C39.3838 5.82008 40.4145 5.52841 41.5348 5.52841C42.3899 5.52841 43.1025 5.67424 43.6726 5.96591C44.2493 6.25095 44.7133 6.6089 45.0646 7.03977C45.4226 7.46401 45.6944 7.88163 45.88 8.29261H46.0092V0.636363H50.2351V21H46.0589V18.554H45.88C45.6811 18.9782 45.3994 19.3991 45.0348 19.8168C44.6768 20.2277 44.2095 20.5691 43.6328 20.8409C43.0627 21.1127 42.3667 21.2486 41.5447 21.2486ZM42.8871 17.8778C43.5698 17.8778 44.1465 17.6922 44.6172 17.321C45.0945 16.9432 45.459 16.4162 45.7109 15.7401C45.9695 15.0639 46.0987 14.2718 46.0987 13.3636C46.0987 12.4555 45.9728 11.6667 45.7209 10.9972C45.469 10.3277 45.1044 9.81061 44.6271 9.44602C44.1499 9.08144 43.5698 8.89915 42.8871 8.89915C42.1911 8.89915 41.6044 9.08807 41.1271 9.46591C40.6499 9.84375 40.2886 10.3674 40.0433 11.0369C39.7981 11.7064 39.6754 12.482 39.6754 13.3636C39.6754 14.2519 39.7981 15.0374 40.0433 15.7202C40.2952 16.3963 40.6565 16.9266 41.1271 17.3111C41.6044 17.6889 42.1911 17.8778 42.8871 17.8778ZM54.3327 26.7273C53.7958 26.7273 53.292 26.6842 52.8214 26.598C52.3574 26.5185 51.9729 26.4157 51.668 26.2898L52.6225 23.1278C53.1197 23.2803 53.5671 23.3632 53.9648 23.3764C54.3692 23.3897 54.7172 23.2969 55.0089 23.098C55.3072 22.8991 55.5491 22.5611 55.7347 22.0838L55.9833 21.4375L50.5046 5.72727H54.9592L58.1211 16.9432H58.2802L61.4719 5.72727H65.9563L60.0202 22.6506C59.7352 23.4725 59.3474 24.1884 58.8569 24.7983C58.373 25.4148 57.7598 25.8887 57.0174 26.2202C56.275 26.5582 55.3801 26.7273 54.3327 26.7273ZM70.6839 21V0.636363H78.718C80.2625 0.636363 81.5784 0.931344 82.6655 1.52131C83.7526 2.10464 84.5812 2.91667 85.1513 3.95739C85.728 4.99148 86.0163 6.18466 86.0163 7.53693C86.0163 8.8892 85.7247 10.0824 85.1413 11.1165C84.558 12.1506 83.7128 12.956 82.6058 13.5327C81.5054 14.1094 80.1731 14.3977 78.6087 14.3977H73.4879V10.9474H77.9126C78.7412 10.9474 79.424 10.8049 79.9609 10.5199C80.5045 10.2282 80.9089 9.82718 81.174 9.31676C81.4458 8.79972 81.5817 8.20644 81.5817 7.53693C81.5817 6.86079 81.4458 6.27083 81.174 5.76704C80.9089 5.25663 80.5045 4.86222 79.9609 4.58381C79.4174 4.29877 78.728 4.15625 77.8928 4.15625H74.9893V21H70.6839ZM86.7333 21V5.72727H90.8398V8.39205H90.9989C91.2773 7.44413 91.7447 6.72822 92.4009 6.24432C93.0572 5.75379 93.8129 5.50852 94.668 5.50852C94.8801 5.50852 95.1088 5.52178 95.354 5.54829C95.5993 5.57481 95.8147 5.61127 96.0004 5.65767V9.41619C95.8015 9.35653 95.5264 9.3035 95.1751 9.2571C94.8237 9.2107 94.5022 9.1875 94.2106 9.1875C93.5875 9.1875 93.0307 9.32339 92.5401 9.59517C92.0562 9.86032 91.6718 10.2315 91.3867 10.7088C91.1083 11.1861 90.9691 11.7363 90.9691 12.3594V21H86.7333ZM102.38 21.2983C100.835 21.2983 99.4998 20.9702 98.3729 20.3139C97.2526 19.651 96.3875 18.7296 95.7777 17.5497C95.1679 16.3632 94.8629 14.9877 94.8629 13.4233C94.8629 11.8456 95.1679 10.4669 95.7777 9.28693C96.3875 8.10038 97.2526 7.17898 98.3729 6.52273C99.4998 5.85985 100.835 5.52841 102.38 5.52841C103.924 5.52841 105.257 5.85985 106.377 6.52273C107.504 7.17898 108.372 8.10038 108.982 9.28693C109.592 10.4669 109.897 11.8456 109.897 13.4233C109.897 14.9877 109.592 16.3632 108.982 17.5497C108.372 18.7296 107.504 19.651 106.377 20.3139C105.257 20.9702 103.924 21.2983 102.38 21.2983ZM102.4 18.017C103.103 18.017 103.689 17.8182 104.16 17.4205C104.63 17.0161 104.985 16.4659 105.224 15.7699C105.469 15.0739 105.592 14.2817 105.592 13.3935C105.592 12.5052 105.469 11.7131 105.224 11.017C104.985 10.321 104.63 9.77083 104.16 9.36648C103.689 8.96212 103.103 8.75994 102.4 8.75994C101.691 8.75994 101.094 8.96212 100.61 9.36648C100.133 9.77083 99.7715 10.321 99.5263 11.017C99.2876 11.7131 99.1683 12.5052 99.1683 13.3935C99.1683 14.2817 99.2876 15.0739 99.5263 15.7699C99.7715 16.4659 100.133 17.0161 100.61 17.4205C101.094 17.8182 101.691 18.017 102.4 18.017Z"
                fill="url(#paint0_linear_3576_1250)"
            />
        </g>
        <defs>
            <filter
                id="filter0_d_3576_1250"
                x="0.412109"
                y="0.35791"
                width="110.485"
                height="28.3694"
                filterUnits="userSpaceOnUse"
                colorInterpolationFilters="sRGB"
            >
                <feFlood floodOpacity="0" result="BackgroundImageFix" />
                <feColorMatrix
                    in="SourceAlpha"
                    type="matrix"
                    values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0"
                    result="hardAlpha"
                />
                <feOffset dy="1" />
                <feGaussianBlur stdDeviation="0.5" />
                <feComposite in2="hardAlpha" operator="out" />
                <feColorMatrix type="matrix" values="0 0 0 0 0.278089 0 0 0 0 0.267405 0 0 0 0 0.267405 0 0 0 0.25 0" />
                <feBlend mode="normal" in2="BackgroundImageFix" result="effect1_dropShadow_3576_1250" />
                <feBlend mode="normal" in="SourceGraphic" in2="effect1_dropShadow_3576_1250" result="shape" />
            </filter>
            <linearGradient
                id="paint0_linear_3576_1250"
                x1="33.1053"
                y1="23.8571"
                x2="47.5294"
                y2="-13.7597"
                gradientUnits="userSpaceOnUse"
            >
                <stop stopColor="#EC4D49" />
                <stop offset="0.491721" stopColor="#7048E8" />
                <stop offset="1" stopColor="#4AC1E8" />
            </linearGradient>
        </defs>
    </svg>
)

export const AutocompletesIcon = (): ReactElement => (
    <svg width="33" height="34" viewBox="0 0 33 34" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect width="33" height="34" rx="16.5" fill="#6B47D6" />
        <rect width="33" height="34" rx="16.5" fill="url(#paint0_linear_2692_3962)" />
        <path
            d="M18.0723 24.8147L14.9142 21.6566L15.9658 20.5943L18.0723 22.7008L22.4826 18.2799L23.5343 19.3421L18.0723 24.8147ZM9.5166 20.1419L13.331 10.2329H14.924L18.7277 20.1419H17.1161L16.1305 17.5438H11.9834L11.0084 20.1419H9.5166ZM12.3829 16.2867H15.7334L14.1079 11.7981H14.006L12.3829 16.2867Z"
            fill="white"
        />
        <defs>
            <linearGradient
                id="paint0_linear_2692_3962"
                x1="16.5"
                y1="0"
                x2="16.5"
                y2="34"
                gradientUnits="userSpaceOnUse"
            >
                <stop stopColor="#FF3424" />
                <stop offset="1" stopColor="#CF275B" />
            </linearGradient>
        </defs>
    </svg>
)

export const ChatMessagesIcon = (): ReactElement => (
    <svg width="34" height="34" viewBox="0 0 34 34" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect x="0.5" width="33" height="34" rx="16.5" fill="#6B47D6" />
        <rect x="0.5" width="33" height="34" rx="16.5" fill="url(#paint0_linear_2692_3970)" />
        <path
            d="M12.4559 18.5188H18.4046V17.3938H12.4559V18.5188ZM12.4559 16.0267H21.544V14.9017H12.4559V16.0267ZM12.4559 13.5533H21.544V12.4283H12.4559V13.5533ZM9.14697 24.8832V10.6683C9.14697 10.2466 9.3022 9.87948 9.61265 9.56695C9.92311 9.25441 10.2877 9.09814 10.7065 9.09814H23.2934C23.7151 9.09814 24.0822 9.25441 24.3948 9.56695C24.7073 9.87948 24.8635 10.2466 24.8635 10.6683V20.2495C24.8635 20.6683 24.7073 21.0329 24.3948 21.3433C24.0822 21.6538 23.7151 21.809 23.2934 21.809H12.2211L9.14697 24.8832ZM11.7035 20.2495H23.2934V10.6683H10.7065V21.359L11.7035 20.2495Z"
            fill="white"
        />
        <defs>
            <linearGradient id="paint0_linear_2692_3970" x1="17" y1="0" x2="17" y2="34" gradientUnits="userSpaceOnUse">
                <stop stopColor="#03C9ED" />
                <stop offset="1" stopColor="#536AEA" />
            </linearGradient>
        </defs>
    </svg>
)

export const TrialPeriodIcon = (): ReactElement => (
    <svg width="34" height="34" viewBox="0 0 34 34" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect x="0.5" width="33" height="34" rx="16.5" fill="#6B47D6" />
        <rect x="0.5" width="33" height="34" rx="16.5" fill="url(#paint0_linear_2898_1552)" />
        <path
            d="M17 27C14.7 27 12.6958 26.2375 10.9875 24.7125C9.27917 23.1875 8.3 21.2833 8.05 19H10.1C10.3333 20.7333 11.1042 22.1667 12.4125 23.3C13.7208 24.4333 15.25 25 17 25C18.95 25 20.6042 24.3208 21.9625 22.9625C23.3208 21.6042 24 19.95 24 18C24 16.05 23.3208 14.3958 21.9625 13.0375C20.6042 11.6792 18.95 11 17 11C15.85 11 14.775 11.2667 13.775 11.8C12.775 12.3333 11.9333 13.0667 11.25 14H14V16H8V10H10V12.35C10.85 11.2833 11.8875 10.4583 13.1125 9.875C14.3375 9.29167 15.6333 9 17 9C18.25 9 19.4208 9.2375 20.5125 9.7125C21.6042 10.1875 22.5542 10.8292 23.3625 11.6375C24.1708 12.4458 24.8125 13.3958 25.2875 14.4875C25.7625 15.5792 26 16.75 26 18C26 19.25 25.7625 20.4208 25.2875 21.5125C24.8125 22.6042 24.1708 23.5542 23.3625 24.3625C22.5542 25.1708 21.6042 25.8125 20.5125 26.2875C19.4208 26.7625 18.25 27 17 27ZM19.8 22.2L16 18.4V13H18V17.6L21.2 20.8L19.8 22.2Z"
            fill="white"
        />
        <defs>
            <linearGradient
                id="paint0_linear_2898_1552"
                x1="17"
                y1="34"
                x2="17"
                y2="-1.57923e-07"
                gradientUnits="userSpaceOnUse"
            >
                <stop stopColor="#F59F00" />
                <stop offset="1" stopColor="#FBD999" />
            </linearGradient>
        </defs>
    </svg>
)
